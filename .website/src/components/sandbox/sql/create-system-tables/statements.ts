// the statement order of createSystemTables -- one entry per Exec in the Go
// method. Both lists below walk that order: the templates as the Go source
// writes them, which the drift test reads, and the statements with the schema
// filled, which the sandbox runs. Keeping them in one file is what makes a
// statement added to only one of them visible.
import { interpolate } from '../interpolate';
import { createSystemConfigSql } from './create-system-config';
import { createStreamConfigSql } from './create-stream-config';
import { createStreamConfigLogSql } from './create-stream-config-log';
import { createStreamConfigLogIndexSql } from './create-stream-config-log-index';
import { createConsumerGroupConfigSql } from './create-consumer-group-config';
import { createWorkerConfigSql } from './create-worker-config';
import { createWorkerConfigStreamNameIndexSql } from './create-worker-config-stream-name-index';
import { createWorkerConfigGroupNameIndexSql } from './create-worker-config-group-name-index';
import { createWorkerConfigSystemNameIndexSql } from './create-worker-config-system-name-index';
import { createWorkerConfigLogSql } from './create-worker-config-log';
import { createWorkerConfigLogIndexSql } from './create-worker-config-log-index';
import { createWorkerInstanceSql } from './create-worker-instance';
import { createWorkerInstanceWorkerIndexSql } from './create-worker-instance-worker-index';
import { createWorkerInstanceExpiryIndexSql } from './create-worker-instance-expiry-index';
import { createScheduleConfigSql } from './create-schedule-config';
import { createScheduleCursorSql } from './create-schedule-cursor';
import { createScheduleCursorDueIndexSql } from './create-schedule-cursor-due-index';
import { createMigrationLogSql } from './create-migration-log';
import { createWorkerInstanceLogExpiryIndexSql } from './create-worker-instance-log-expiry-index';
import { createWorkerInstanceLogWorkerIndexSql } from './create-worker-instance-log-worker-index';
import { createWorkerInstanceLogSql } from './create-worker-instance-log';

export const createSystemTablesTemplates: string[] = [
	createSystemConfigSql,
	createStreamConfigSql,
	createStreamConfigLogSql,
	createStreamConfigLogIndexSql,
	createConsumerGroupConfigSql,
	createWorkerConfigSql,
	createWorkerConfigStreamNameIndexSql,
	createWorkerConfigGroupNameIndexSql,
	createWorkerConfigSystemNameIndexSql,
	createWorkerConfigLogSql,
	createWorkerConfigLogIndexSql,
	createWorkerInstanceSql,
	createWorkerInstanceLogSql,
	createWorkerInstanceWorkerIndexSql,
	createWorkerInstanceExpiryIndexSql,
	createWorkerInstanceLogWorkerIndexSql,
	createWorkerInstanceLogExpiryIndexSql,
	createScheduleConfigSql,
	createScheduleCursorSql,
	createScheduleCursorDueIndexSql,
	createMigrationLogSql,
];

// every statement names only shared tables, so the schema is the one verb to
// fill -- the stream side's sibling takes the stream id as well
export function createSystemTablesStatements(): string[] {
	return createSystemTablesTemplates.map((template) => interpolate(template));
}
