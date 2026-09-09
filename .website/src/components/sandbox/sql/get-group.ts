// verbatim from pkg/consume/controller/datastore/group.go getGroup
import { interpolate } from './interpolate';

export const getGroupSqlTemplate = `
		-- sqlstreams: consume.getGroup
		SELECT id, stream_id, name, created_at
		FROM %[1]s.consumer_group_config
		WHERE stream_id = $1 AND name = $2;
	`;

export function getGroupSql(): string {
	return interpolate(getGroupSqlTemplate);
}
