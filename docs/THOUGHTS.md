# Public API

05 - head, err := configs.GetCompactionHeadInTx(ctx, tx, "dev-7")
this feels out of place like it should be done in a different way
problem is we established this get pattern client.Topic[DeviceConfig](registered.Name).Key("dev-7").CompactionHead(ctx)
but then don't use the same shape

08 - we don't need anymore as they have been combined

10 - need to look at this more and understand it. The code shape looks fine but the thing we are trying to show is not
necessarly apparent from just the code

12 - same deal as 10. Code shape looks fine but I think we can likely organize the code to better show what we are trying
to convey

13 - the config field names here are not good
	// newest declaration wins: every minute instead of the @hourly default
	if err := client.System().Register(ctx, &vulkan.RegisterSystemConfig{
		PartitionCount:     &alert.PartitionCountJobConfig{Expression: "* * * * *"},
		CompactionReadCost: &alert.CompactionReadCostJobConfig{Expression: "* * * * *"},
		WorkerLiveness:     &alert.WorkerLivenessJobConfig{Expression: "* * * * *"},
	}); err != nil {
		return err
	}

# Docs

Make small little comment about client api being design for old timers who still like to hand write code occassionally
- ie can easily discover through dot tree notation what is available

# Review

Probably should have one more table name and column review (this will be hard to change later)

should probably look to see if we could speed up claim query its gotten unruly with ctes and conditionals

need to make sure we do some manual testing for cli, metrics and alerts

need another review to make sure we are not logging any sensitive information like payload

# Other

