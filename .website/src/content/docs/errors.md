---
title: Error codes
---

Every SQLStreams error carries a stable `SS` code. The code renders at the
end of the one-liner (`[SS0005]`), in JSON logs, and in the CLI's error
block; paste the message text or code into search to land on its page.

| Code | Problem | Recovery |
| ---- | ------- | -------- |
| [SS0001](/errors/SS0001) | instance is already consuming | permanent |
| [SS0002](/errors/SS0002) | lifecycle context can never be cancelled | permanent |
| [SS0003](/errors/SS0003) | lease lost to another consumer | permanent |
| [SS0004](/errors/SS0004) | stream partition size does not match the existing stream | permanent |
| [SS0005](/errors/SS0005) | stream not found | permanent |
| [SS0006](/errors/SS0006) | stream still holds messages | permanent |
| [SS0007](/errors/SS0007) | stream name already taken | permanent |
| [SS0008](/errors/SS0008) | destroy is disabled | permanent |
| [SS0009](/errors/SS0009) | stream name uses the reserved __system. prefix | permanent |
| [SS0010](/errors/SS0010) | a worker instance is still live | permanent |
| [SS0011](/errors/SS0011) | streams are still registered | permanent |
| [SS0012](/errors/SS0012) | worker instance row expired or was removed | permanent |
| [SS0013](/errors/SS0013) | schedule not found | permanent |
| [SS0014](/errors/SS0014) | consumer group not found | permanent |
| [SS0015](/errors/SS0015) | consumer group still has a live consumer | permanent |
| [SS0016](/errors/SS0016) | consumer group still has delivery rows | permanent |
| [SS0017](/errors/SS0017) | system not registered | permanent |
| [SS0018](/errors/SS0018) | could not create the covering partition | transient |
| [SS0019](/errors/SS0019) | commit confirmation was lost | permanent |
| [SS0020](/errors/SS0020) | stream partitions remain after draining | permanent |
| [SS0021](/errors/SS0021) | could not finish the stream declaration | transient |
| [SS0022](/errors/SS0022) | schema version is older than this build requires | permanent |
| [SS0023](/errors/SS0023) | schema version is newer than this build understands | permanent |
| [SS0024](/errors/SS0024) | could not finish the worker declaration | transient |
| [SS0025](/errors/SS0025) | could not finish the schedule declaration | transient |
| [SS0053](/errors/SS0053) | could not take a lock needed by the migration step | transient |
| [SS0056](/errors/SS0056) | partition creation cannot keep up with the id sequence | permanent |

Log events share the same `SS` code space: a Warn- or Error-level line
that is operator-actionable carries its code in the line's `code` attribute,
and the code lands on a page here the same way.

| Code | Event | Level |
| ---- | ----- | ----- |
| [SS0026](/errors/SS0026) | lease reclaimed from expired worker | warn |
| [SS0027](/errors/SS0027) | range quarantined after max reclaims | warn |
| [SS0028](/errors/SS0028) | messages dead-lettered | warn |
| [SS0029](/errors/SS0029) | message dead-lettered | warn |
| [SS0030](/errors/SS0030) | exception dead-lettered | warn |
| [SS0031](/errors/SS0031) | crash-loop kill backstop fired | warn |
| [SS0032](/errors/SS0032) | stored message options outside this consumer's bounds | warn |
| [SS0033](/errors/SS0033) | could not create partition ahead | warn |
| [SS0057](/errors/SS0057) | no partition covers the next message id | warn |
| [SS0058](/errors/SS0058) | schedule target stream keeps no success rows | warn |
| [SS0034](/errors/SS0034) | worker instance lost | warn |
| [SS0035](/errors/SS0035) | manager row suspended | warn |
| [SS0036](/errors/SS0036) | worker tick backoff curve exhausted | error |
| [SS0037](/errors/SS0037) | schedule message was already produced by an earlier ambiguous commit | warn |
| [SS0038](/errors/SS0038) | produce exceeded the duration threshold | warn |
| [SS0039](/errors/SS0039) | delivery dispatch exceeded the duration threshold | warn |
| [SS0040](/errors/SS0040) | worker tick exceeded its poll rate | warn |
| [SS0041](/errors/SS0041) | consumer stopped | info |
| [SS0052](/errors/SS0052) | abandoned-routine events dropped | warn |
| [SS0065](/errors/SS0065) | system manager stopped | error |

Declared metrics share the code space too: a measurement's name resolves
to its declaration, and `sqlstreams explain` accepts the code, the full
name, or the stop-line attribute key (`ready_count`).

| Code | Metric | Kind |
| ---- | ------ | ---- |
| [SS0042](/errors/SS0042) | sqlstreams.consumer.session.claimed | counter |
| [SS0043](/errors/SS0043) | sqlstreams.consumer.session.success | counter |
| [SS0044](/errors/SS0044) | sqlstreams.consumer.session.superseded | counter |
| [SS0045](/errors/SS0045) | sqlstreams.consumer.session.ready | counter |
| [SS0046](/errors/SS0046) | sqlstreams.consumer.session.deferred | counter |
| [SS0047](/errors/SS0047) | sqlstreams.consumer.session.dead | counter |
| [SS0048](/errors/SS0048) | sqlstreams.consumer.session.reclaimed | counter |
| [SS0049](/errors/SS0049) | sqlstreams.consumer.session.quarantined | counter |
| [SS0050](/errors/SS0050) | sqlstreams.consumer.session.abandoned | counter |
| [SS0051](/errors/SS0051) | sqlstreams.consumer.session.lease_lost | counter |
