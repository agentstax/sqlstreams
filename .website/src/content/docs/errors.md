---
title: Error codes
---

Every SQLStreams error carries a stable `SQL` code. The code renders at the
end of the one-liner (`[SQL0005]`), in JSON logs, and in the CLI's error
block; paste the message text or code into search to land on its page.

| Code | Problem | Recovery |
| ---- | ------- | -------- |
| [SQL0001](/errors/SQL0001) | instance is already consuming | permanent |
| [SQL0002](/errors/SQL0002) | lifecycle context can never be cancelled | permanent |
| [SQL0003](/errors/SQL0003) | lease lost to another consumer | permanent |
| [SQL0004](/errors/SQL0004) | stream partition size does not match the existing stream | permanent |
| [SQL0005](/errors/SQL0005) | stream not found | permanent |
| [SQL0006](/errors/SQL0006) | stream still holds messages | permanent |
| [SQL0007](/errors/SQL0007) | stream name already taken | permanent |
| [SQL0008](/errors/SQL0008) | destroy is disabled | permanent |
| [SQL0009](/errors/SQL0009) | stream name uses the reserved __system. prefix | permanent |
| [SQL0010](/errors/SQL0010) | a worker instance is still live | permanent |
| [SQL0011](/errors/SQL0011) | streams are still registered | permanent |
| [SQL0012](/errors/SQL0012) | worker instance row expired or was removed | permanent |
| [SQL0013](/errors/SQL0013) | schedule not found | permanent |
| [SQL0014](/errors/SQL0014) | consumer group not found | permanent |
| [SQL0015](/errors/SQL0015) | consumer group still has a live consumer | permanent |
| [SQL0016](/errors/SQL0016) | consumer group still has delivery rows | permanent |
| [SQL0017](/errors/SQL0017) | system not registered | permanent |
| [SQL0018](/errors/SQL0018) | could not create the covering partition | transient |
| [SQL0019](/errors/SQL0019) | commit confirmation was lost | permanent |
| [SQL0020](/errors/SQL0020) | stream partitions remain after draining | permanent |
| [SQL0021](/errors/SQL0021) | could not finish the stream declaration | transient |
| [SQL0022](/errors/SQL0022) | schema version is older than this build requires | permanent |
| [SQL0023](/errors/SQL0023) | schema version is newer than this build understands | permanent |
| [SQL0024](/errors/SQL0024) | could not finish the worker declaration | transient |
| [SQL0025](/errors/SQL0025) | could not finish the schedule declaration | transient |
| [SQL0053](/errors/SQL0053) | could not take a lock needed by the migration step | transient |
| [SQL0056](/errors/SQL0056) | partition creation cannot keep up with the id sequence | permanent |

Log events share the same `SQL` code space: a Warn- or Error-level line
that is operator-actionable carries its code in the line's `code` attribute,
and the code lands on a page here the same way.

| Code | Event | Level |
| ---- | ----- | ----- |
| [SQL0026](/errors/SQL0026) | lease reclaimed from expired worker | warn |
| [SQL0027](/errors/SQL0027) | range quarantined after max reclaims | warn |
| [SQL0028](/errors/SQL0028) | messages dead-lettered | warn |
| [SQL0029](/errors/SQL0029) | message dead-lettered | warn |
| [SQL0030](/errors/SQL0030) | exception dead-lettered | warn |
| [SQL0031](/errors/SQL0031) | crash-loop kill backstop fired | warn |
| [SQL0032](/errors/SQL0032) | stored message options outside this consumer's bounds | warn |
| [SQL0033](/errors/SQL0033) | could not create partition ahead | warn |
| [SQL0057](/errors/SQL0057) | no partition covers the next message id | warn |
| [SQL0058](/errors/SQL0058) | schedule target stream keeps no success rows | warn |
| [SQL0034](/errors/SQL0034) | worker instance lost | warn |
| [SQL0035](/errors/SQL0035) | manager row suspended | warn |
| [SQL0036](/errors/SQL0036) | worker tick backoff curve exhausted | error |
| [SQL0037](/errors/SQL0037) | schedule message was already produced by an earlier ambiguous commit | warn |
| [SQL0038](/errors/SQL0038) | produce exceeded the duration threshold | warn |
| [SQL0039](/errors/SQL0039) | delivery dispatch exceeded the duration threshold | warn |
| [SQL0040](/errors/SQL0040) | worker tick exceeded its poll rate | warn |
| [SQL0041](/errors/SQL0041) | consumer stopped | info |
| [SQL0052](/errors/SQL0052) | abandoned-routine events dropped | warn |
| [SQL0065](/errors/SQL0065) | system manager stopped | error |

Declared metrics share the code space too: a measurement's name resolves
to its declaration, and `sqlstreams explain` accepts the code, the full
name, or the stop-line attribute key (`ready_count`).

| Code | Metric | Kind |
| ---- | ------ | ---- |
| [SQL0042](/errors/SQL0042) | sqlstreams.consumer.session.claimed | counter |
| [SQL0043](/errors/SQL0043) | sqlstreams.consumer.session.success | counter |
| [SQL0044](/errors/SQL0044) | sqlstreams.consumer.session.superseded | counter |
| [SQL0045](/errors/SQL0045) | sqlstreams.consumer.session.ready | counter |
| [SQL0046](/errors/SQL0046) | sqlstreams.consumer.session.deferred | counter |
| [SQL0047](/errors/SQL0047) | sqlstreams.consumer.session.dead | counter |
| [SQL0048](/errors/SQL0048) | sqlstreams.consumer.session.reclaimed | counter |
| [SQL0049](/errors/SQL0049) | sqlstreams.consumer.session.quarantined | counter |
| [SQL0050](/errors/SQL0050) | sqlstreams.consumer.session.abandoned | counter |
| [SQL0051](/errors/SQL0051) | sqlstreams.consumer.session.lease_lost | counter |
