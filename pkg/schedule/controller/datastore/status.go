package datastore

import (
	"context"
	"errors"
	"fmt"

	"github.com/allegedlyreliable/sqlstreams/pkg/stream"
	"github.com/jackc/pgx/v5"
)

// Status is one ScheduleConsumerGroupSummaryRow per consumer group that receives the
// schedule's messages. Counts cover the stream's retention window.
func (d *ScheduleDatastore) Status(ctx context.Context, streamId int64, name string) ([]ScheduleConsumerGroupSummaryRow, error) {
	var statuses []ScheduleConsumerGroupSummaryRow
	err := d.DatastoreRetry.WrapIdempotent(ctx, func() error {
		var err error
		statuses, err = d.status(ctx, streamId, name)
		return err
	})
	return statuses, err
}

func (d *ScheduleDatastore) status(ctx context.Context, streamId int64, name string) ([]ScheduleConsumerGroupSummaryRow, error) {
	groups, err := d.matchingGroups(ctx, streamId, name)
	if err != nil {
		return nil, err
	}
	messageIds, err := d.keyMessageIds(ctx, streamId, name)
	if err != nil {
		return nil, err
	}
	headId, err := d.headId(ctx, streamId, name)
	if err != nil {
		return nil, err
	}

	var statuses []ScheduleConsumerGroupSummaryRow
	for _, group := range groups {
		outcomes, err := d.messageOutcomes(ctx, streamId, group.Id, messageIds)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, scheduleConsumerGroupSummary(group, messageIds, headId, outcomes))
	}
	return statuses, nil
}

// matchingGroups is every consumer group that receives the schedule's requests,
// ordered by name.
func (d *ScheduleDatastore) matchingGroups(ctx context.Context, streamId int64, name string) ([]matchingGroupRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: schedule.matchingGroups
		SELECT cg.id, cg.name
		FROM %[1]s.consumer_group_config cg
		WHERE cg.stream_id = $1
		  AND (
			-- a group with no bindings receives every routing key
			NOT EXISTS (SELECT 1 FROM %[1]s.%[2]s b WHERE b.consumer_group_id = cg.id)
			-- otherwise a binding must match the found's name ($2)
			OR EXISTS (SELECT 1 FROM %[1]s.%[2]s b WHERE b.consumer_group_id = cg.id AND $2 ~ b.pattern_regex)
		  )
		ORDER BY cg.name;
	`, d.Datastore.Schema, stream.BindingConfigTable(streamId))
	rows, err := d.Datastore.Pool.Query(ctx, sql, streamId, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []matchingGroupRow
	for rows.Next() {
		var group matchingGroupRow
		if err := rows.Scan(&group.Id, &group.Name); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

// keyMessageIds is every message id on the schedule's message key still inside
// the retention window.
func (d *ScheduleDatastore) keyMessageIds(ctx context.Context, streamId int64, name string) ([]int64, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: schedule.keyMessageIds
		SELECT m.id
		FROM %[1]s.%[2]s m
		WHERE m.message_key = $1;
	`, d.Datastore.Schema, stream.MessageLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messageIds []int64
	for rows.Next() {
		var messageId int64
		if err := rows.Scan(&messageId); err != nil {
			return nil, err
		}
		messageIds = append(messageIds, messageId)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return messageIds, nil
}

// headId is the key's compaction_head pointer; 0 when the schedule has no messages.
func (d *ScheduleDatastore) headId(ctx context.Context, streamId int64, name string) (int64, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: schedule.headId
		SELECT message_id
		FROM %[1]s.%[2]s
		WHERE compaction_key = $1
			AND message_id IS NOT NULL;
	`, d.Datastore.Schema, stream.CompactionHeadTable(streamId))

	var headId int64
	err := d.Datastore.Pool.QueryRow(ctx, sql, name).Scan(&headId)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return headId, nil
}

// messageOutcomes is one consumer group's delivery history per message,
// rolled up to booleans and indexed by message id.
func (d *ScheduleDatastore) messageOutcomes(ctx context.Context, streamId int64, consumerGroupId int64, messageIds []int64) (map[int64]messageOutcomeRow, error) {
	sql := fmt.Sprintf(`
		-- sqlstreams: schedule.messageOutcomes
		SELECT
			d.message_id,
			bool_or(d.status = 'success')                          AS succeeded,
			bool_or(d.status IN ('failure', 'expired', 'killed'))  AS raised,
			bool_or(d.status = 'deferred')                         AS deferred
		FROM %[1]s.%[2]s d
		WHERE d.consumer_group_id = $1
		  AND d.message_id = ANY($2)
		GROUP BY d.message_id;
	`, d.Datastore.Schema, stream.DeliveryLogTable(streamId))

	rows, err := d.Datastore.Pool.Query(ctx, sql, consumerGroupId, messageIds)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	outcomes := make(map[int64]messageOutcomeRow)
	for rows.Next() {
		var messageId int64
		var outcome messageOutcomeRow
		if err := rows.Scan(&messageId, &outcome.Succeeded, &outcome.Raised, &outcome.Deferred); err != nil {
			return nil, err
		}
		outcomes[messageId] = outcome
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return outcomes, nil
}

// ***************
// *** HELPERS ***
// ***************

// 'superseded' and still-pending 'deferred' messages never
// ran, so ran = succeeded + failed always holds
func scheduleConsumerGroupSummary(group matchingGroupRow, messageIds []int64, headId int64, outcomes map[int64]messageOutcomeRow) ScheduleConsumerGroupSummaryRow {
	status := ScheduleConsumerGroupSummaryRow{ConsumerGroup: group.Name}
	for _, messageId := range messageIds {
		outcome := outcomes[messageId]
		switch {
		case outcome.Succeeded:
			status.Ran++
			status.Succeeded++
		case outcome.Raised:
			status.Ran++
			status.Failed++
		// dropped unrun: no longer the key's head and this group never ran it
		case messageId != headId:
			status.Superseded++
		}
	}
	return status
}
