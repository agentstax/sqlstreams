# Public API

09 and 11 - need to review again and make sure code makes sense and clearly outlines what it intends to describe.

should ideally come up with one or two 'fake scenarios' that all the example playgrounds can follow so
it is conceptually easier to follow along each

# Docs

Make small little comment about client api being design for old timers who still like to hand write code occassionally
- ie can easily discover through dot tree notation what is available

# Review

Probably should have one more table name and column review (this will be hard to change later)

should probably look to see if we could speed up claim query its gotten unruly with ctes and conditionals

need to make sure we do some manual testing for cli, metrics and alerts

need another review to make sure we are not logging any sensitive information like payload

# Other

janitor sweep step need to be non blocking with a timeout context, otherwise can get blocked
func (i *JanitorInstance) sweep(ctx context.Context) error {
	current := i.Topic
	if err := i.controller.DropExpiredPartitions(ctx, current.Id, current.PartitionSize, current.RetentionTTL, current.AllowDropPastCommitted, current.DeliveryLogMode); err != nil {
		return err
	}
	if err := i.controller.SweepExpiredPartitions(ctx, current.Id, current.PartitionSize, current.RetentionTTL, current.AllowDropPastCommitted, i.metadata.SweepBatchSize, current.DeliveryLogMode); err != nil {
		return err
	}
	if err := i.controller.SweepExpiredIdempotencyKeys(ctx, current.Id, current.IdempotencyKeyTTL, i.metadata.SweepBatchSize); err != nil {
		return err
	}
	if err := i.controller.SweepExpiredEmptyCompactionHeads(ctx, current.Id, current.EmptyCompactionHeadTTL, i.metadata.SweepBatchSize); err != nil {
		return err
	}
	return i.controller.SweepExpiredKeyLeases(ctx, current.Id, i.metadata.SweepBatchSize)
}


