# Public API

09 and 11 - need to review again and make sure code makes sense and clearly outlines what it intends to describe.

# Docs

cleanup and split out the website docs. They have grown a bit unruly and I much prefer smaller bite sized doc pages.

cleanup root documents and /docs documents (mostly root docs like AGENT.md)

roadmap later item for review code for interesting design decisions to write articles on (big or small things)
- diagnostic code system
- suppression logger
- doc site
- client builder pattern choice
- writing your own commit messages (and small commit size)
- Decision tracking and index
- Code quality is even MORE important now
  - bad code and patterns snowball, just as good code and patterns do
  - stray unused code of patterns dilutes context 
- AI Native projects
  - faster but lose context
  - you can learn but its worse and must be disciplined? (does learning even matter)
  - Is it right or fair?
  - How could this be maintained long term
  - amount of corrections for frontier models (50%)
  - When to heavily review the code vs skim or not at all (what is the right balance)
- The importance of keeping notes (previously the slower pace would allow for easier tracking, things change so quick now it is easier to forget)
- rules vs conventions and the tradeoffs of each
  - rules are enforced via code, scripts etc. Have maintainenance and overly aggressive rules can be annoying and brittle
  - conventions easy and work well with workflows but easily accumlate drift overtime (if large enough project)

# Review

should probably look to see if we could speed up claim query its gotten unruly with ctes and conditionals

## Manual

Probably should have one more table name and column review (this will be hard to change later)

need to make sure we do some manual testing for cli, metrics and alerts

manual review of public user facing comments :(. I don't want to but its got to be done

# Other
