# Selection from tools/design/runs/2026-09-27-propose-1/pool.json

## Starting point

Before the search, the current knowledge base was brought up to date:

- refreshed **cloud_form** from the pool: settled values, fitted error rates
- refreshed **sun_view** from the pool: settled values, fitted error rates
- refreshed **falling_visible** from the pool: settled values, fitted error rates
- refitted **base**: noise 0.20 → 0.20, answer rate 99%
- refitted **colour**: noise 0.02 → 0.02, answer rate 100%
- refitted **depth**: noise 0.08 → 0.08, answer rate 91%
- refitted **element_size**: noise 0.27 → 0.27, answer rate 93%
- refitted **halo**: noise 0.17 → 0.17, answer rate 100%
- refitted **hooks**: noise 0.30 → 0.30, answer rate 100%
- refitted **mamma**: noise 0.20 → 0.20, answer rate 100%
- refitted **shading**: noise 0.29 → 0.29, answer rate 97%
- refitted **sky_cover**: noise 0.07 → 0.07, answer rate 100%
- refitted **top**: noise 0.07 → 0.07, answer rate 97%
- refitted **turrets**: noise 0.24 → 0.25, answer rate 87%

Before: identified 60%, 8.6 questions, 301 words read, gave up 0%: score -65.7  
After: identified 60%, 8.6 questions, 301 words read, gave up 0%: score -65.7

Score: percent identified, less 5 points per 12 words read (a short yes/no question), in games answered with each question's error rates. Moves are screened on a few games per entity and taken only if the gain holds up on many more; the scores here are those.

No move raised the score; the candidate is the starting point, as brought up to date if it was.

## Promising on screening, not confirmed

| move | identified | questions | words read | score |
| --- | --- | --- | --- | --- |
| drop mamma | 58% | 8.4 | 302 | -67.9 |
| drop element_size | 58% | 8.1 | 284 | -60.5 |
| drop base | 58% | 8.2 | 287 | -61.9 |

## Questions over the limit of 5 answers

These stay only because nothing shorter yet does their work; the next proposals should replace them.

- cloud_form: 8 answers

## Proposals never tried

- cloud_form: 8 options, more than 5
