// the statement order of createStreamTables -- one entry per Exec in the Go
// method. Both lists below walk that order: the templates as the Go source
// writes them, which the drift test reads, and the statements with the schema
// and table names filled, which the sandbox runs. Keeping them in one file is
// what makes a statement added to only one of them visible.
import { createTableSql, createTableSqlTemplate } from './create-table';
import { createPartitionSql, createPartitionSqlTemplate } from './create-partition';
import {
	createMessageKeyIndexSql,
	createMessageKeyIndexSqlTemplate,
} from './create-message-key-index';
import { createIdempotencyKeySql, createIdempotencyKeySqlTemplate } from './create-idempotency-key';
import {
	createIdempotencyKeyCreatedAtIndexSql,
	createIdempotencyKeyCreatedAtIndexSqlTemplate,
} from './create-idempotency-key-created-at-index';
import { createExceptionQueueSql, createExceptionQueueSqlTemplate } from './create-exception-queue';
import {
	createExceptionQueueMessageKeyIndexSql,
	createExceptionQueueMessageKeyIndexSqlTemplate,
} from './create-exception-queue-message-key-index';
import { createDeliveryLogSql, createDeliveryLogSqlTemplate } from './create-delivery-log';
import {
	createDeliveryLogAttemptIndexSql,
	createDeliveryLogAttemptIndexSqlTemplate,
} from './create-delivery-log-attempt-index';
import {
	createConsumerGroupCursorSql,
	createConsumerGroupCursorSqlTemplate,
} from './create-consumer-group-cursor';
import { createClaimLeaseSql, createClaimLeaseSqlTemplate } from './create-claim-lease';
import {
	createMessageKeyLeaseSql,
	createMessageKeyLeaseSqlTemplate,
} from './create-message-key-lease';
import { createCompactionHeadSql, createCompactionHeadSqlTemplate } from './create-compaction-head';
import {
	createCompactionHeadEmptyUpdatedAtIndexSql,
	createCompactionHeadEmptyUpdatedAtIndexSqlTemplate,
} from './create-compaction-head-empty-updated-at-index';
import { createBindingConfigSql, createBindingConfigSqlTemplate } from './create-binding-config';
import {
	createBindingConfigLogSql,
	createBindingConfigLogSqlTemplate,
} from './create-binding-config-log';
import {
	createBindingConfigLogIndexSql,
	createBindingConfigLogIndexSqlTemplate,
} from './create-binding-config-log-index';

export const createStreamTablesTemplates: string[] = [
	createTableSqlTemplate,
	createPartitionSqlTemplate,
	createMessageKeyIndexSqlTemplate,
	createIdempotencyKeySqlTemplate,
	createIdempotencyKeyCreatedAtIndexSqlTemplate,
	createExceptionQueueSqlTemplate,
	createExceptionQueueMessageKeyIndexSqlTemplate,
	createDeliveryLogSqlTemplate,
	createDeliveryLogAttemptIndexSqlTemplate,
	createConsumerGroupCursorSqlTemplate,
	createClaimLeaseSqlTemplate,
	createMessageKeyLeaseSqlTemplate,
	createCompactionHeadSqlTemplate,
	createCompactionHeadEmptyUpdatedAtIndexSqlTemplate,
	createBindingConfigSqlTemplate,
	createBindingConfigLogSqlTemplate,
	createBindingConfigLogIndexSqlTemplate,
];

export function createStreamTablesStatements(streamId: number, partitionSize: number): string[] {
	return [
		createTableSql(streamId),
		createPartitionSql(streamId, partitionSize),
		createMessageKeyIndexSql(streamId),
		createIdempotencyKeySql(streamId),
		createIdempotencyKeyCreatedAtIndexSql(streamId),
		createExceptionQueueSql(streamId),
		createExceptionQueueMessageKeyIndexSql(streamId),
		createDeliveryLogSql(streamId),
		createDeliveryLogAttemptIndexSql(streamId),
		createConsumerGroupCursorSql(streamId),
		createClaimLeaseSql(streamId),
		createMessageKeyLeaseSql(streamId),
		createCompactionHeadSql(streamId),
		createCompactionHeadEmptyUpdatedAtIndexSql(streamId),
		createBindingConfigSql(streamId),
		createBindingConfigLogSql(streamId),
		createBindingConfigLogIndexSql(streamId),
	];
}
