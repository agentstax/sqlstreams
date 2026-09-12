# Public API

# Docs

Should add a nerd 'But what about!' note or section in doc for when obvious point or counter argument pops up to address it

the longer users stay on my user profile page (or if they don't move mouse) all other elements besides the scrolling text should start to fade to background or static
and the scrolling text should slowly get larger and larger

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
  - skimmable code (ie readable) is more important than ever
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
  - unit tests are dead (few small mistakes with llm, id vs name)
  - regression tests from debug sessions
- A new world and the case of low dependencies
- the rot within (VPs and middle managers), fuck em those shady ass mf
- When to use kafka vs sqlstreams
  - kafka is far more efficient
- The difference between Opus 4.6 and Fable 5.1 is both HUGE and miniscule
  - its ability to one shot things and solve for inputs and outputs
  - but its effect on day to day maintaining and extending code is minimal
- Showing a feature coding session with setup and prompts and how much back and forth there is
  - would need snapshots of git diffs after each prompt or manual change
- why topic per table instead of single table with LIST and RANGE subpartitions (0757 decision)
- why cursor claim ranges, instead of singular or bit map with holes

# Review

## Manual

Probably should have one more table name and column review (this will be hard to change later)

need to make sure we do some manual testing for cli, metrics and alerts

manual review of public user facing comments :(. I don't want to but its got to be done

review of most important website docs

# Other

Need new github org name "getstweamy"?

