# Selection from tools/design/runs/2026-09-27-propose-1/pool.json

## Starting point

Before the search, the current knowledge base was brought up to date:

- refreshed **cloud_form** from the pool: settled values, fitted error rates
- refreshed **sun_view** from the pool: settled values, fitted error rates
- refreshed **falling_visible** from the pool: settled values, fitted error rates
- refitted **base**: noise 0.12 → 0.20, answer rate 99%
- refitted **colour**: noise 0.12 → 0.02, answer rate 100%
- refitted **depth**: noise 0.12 → 0.08, answer rate 91%
- refitted **element_size**: noise 0.12 → 0.27, answer rate 93%
- refitted **halo**: noise 0.06 → 0.17, answer rate 100%
- refitted **hooks**: noise 0.10 → 0.30, answer rate 100%
- refitted **mamma**: noise 0.08 → 0.20, answer rate 100%
- refitted **shading**: noise 0.12 → 0.29, answer rate 97%
- refitted **sky_cover**: noise 0.12 → 0.07, answer rate 100%
- refitted **top**: noise 0.10 → 0.07, answer rate 97%
- refitted **turrets**: noise 0.10 → 0.25, answer rate 87%

Before: identified 60%, 8.5 questions, gave up 0%: score 17.6  
After: identified 60%, 8.5 questions, gave up 0%: score 17.6

Score: percent identified minus 5 per question, in games answered with each question's error rates. Moves are screened on a few games per entity and taken only if the gain holds up on many more; the scores here are those.

No move raised the score; the candidate is the starting point, as brought up to date if it was.

## Promising on screening, not confirmed

| move | identified | questions | score |
| --- | --- | --- | --- |
| drop mamma | 59% | 8.2 | 17.9 |
| drop top | 59% | 8.4 | 17.1 |
| drop shading | 60% | 8.0 | 19.7 |
