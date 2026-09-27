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
- refitted **hooks**: noise 0.10 → 0.35, answer rate 93%
- refitted **mamma**: noise 0.08 → 0.20, answer rate 100%
- refitted **shading**: noise 0.12 → 0.29, answer rate 97%
- refitted **sky_cover**: noise 0.12 → 0.07, answer rate 100%
- refitted **top**: noise 0.10 → 0.07, answer rate 97%
- refitted **turrets**: noise 0.10 → 0.27, answer rate 99%

Before: identified 62%, 8.5 questions, gave up 1%: score 19.5  
After: identified 66%, 8.4 questions, gave up 0%: score 23.8

Score: percent identified minus 5 per question, in games answered with each question's error rates.

## Moves, in the order taken

| move | identified | questions | score |
| --- | --- | --- | --- |
| add veil_texture | 66% | 8.4 | 23.8 |

## Questions added

- **veil_texture**: Was the sky covered by a thin whitish veil - one you could still see blue through - and if so, did it have any texture?

## Values for a person to settle

These kept the proposer's value in the candidate. Settle them with `ribice-design consult -tasks settle.jsonl -expert human`.

- **veil_texture**: Cirrocumulus stratiformis (not_a_veil)
