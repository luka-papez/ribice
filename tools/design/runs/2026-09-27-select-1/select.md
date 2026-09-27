# Selection from tools/design/runs/2026-09-27-propose-1/pool.json

Before: identified 48%, 11.1 questions, gave up 0%: score -7.0  
After: identified 65%, 9.9 questions, gave up 0%: score 15.5

Score: percent identified minus 5 per question, in games answered with each question's error rates.

## Moves, in the order taken

| move | identified | questions | score |
| --- | --- | --- | --- |
| replace opacity with sun_view | 54% | 10.5 | 1.3 |
| replace shape with cloud_form | 58% | 10.3 | 6.5 |
| replace precipitation with falling_visible | 63% | 10.3 | 11.4 |
| drop anvil | 65% | 9.9 | 15.5 |

## Questions added

- **sun_view**: What did the sun (or moon) look like where this cloud passed in front of it? (replaces opacity)
- **cloud_form**: Which of these was it most like? (replaces shape)
- **falling_visible**: Could you see anything falling out of it? (replaces precipitation)

With default error rates, for want of perceive answers: sun_view, falling_visible.

## Values for a person to settle

These kept the proposer's value in the candidate. Settle them with `ribice-design consult -tasks settle.jsonl -expert human`.

- **sun_view**: Cirrus spissatus (pale_blur); Cirrocumulus lenticularis (sharp_disc); Cirrocumulus floccus (sharp_disc); Cirrocumulus castellanus (sharp_disc); Cirrostratus fibratus (halo_ring); Altocumulus lenticularis (pale_blur); Altocumulus castellanus (pale_blur); Altocumulus floccus (pale_blur); Altocumulus volutus (pale_blur); Stratocumulus lenticularis (blotted_out); Stratocumulus castellanus (blotted_out); Stratocumulus floccus (blotted_out); Stratocumulus volutus (blotted_out); Stratus nebulosus (sharp_disc); Cumulus humilis (blotted_out); Cumulus mediocris (blotted_out); Cumulus fractus (blotted_out)
- **cloud_form**: Cirrus spissatus (smooth_sheet); Cumulus humilis (heap)
- **falling_visible**: Cirrus uncinus (trails_in_air); Cirrus spissatus (nothing); Cirrus castellanus (nothing); Stratocumulus floccus (nothing); Cumulus congestus (shafts_to_ground)
