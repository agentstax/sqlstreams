# Public API

# Docs

tree:
  .examples -> examples (more visible to people peeking at repo)
  .e2e -> .tests/e2e
  .tests -> .tests/integration

website user icon for me needs to change (maybe my shipt profile pic)

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
- Hitting a flow state with two sessions. The new heads down coding joy. Thought I lost the joyouse moment but it still does feel good.
- If you are not cursing out your llms I am concerned about your coding capabilities
- the evolution and stages of testing
  - should you start out with unit tests and increase token costs or wait till code is closer to finalization
  - what are valuable tests in the agent era
  - is testing validation logic valuable, setting up integration test that you don't understand?
- A new world and the case of low dependencies

# Review

## Manual

Probably should have one more table name and column review (this will be hard to change later)

need to make sure we do some manual testing for cli, metrics and alerts

manual review of public user facing comments :(. I don't want to but its got to be done

review of most important website docs

# Other

Another automated review for broken links and inconsistent references