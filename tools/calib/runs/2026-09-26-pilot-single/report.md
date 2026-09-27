# clouds: pilot-single (single mode, claude-opus-5)

## Headline

Right: the true cloud ranked first. Genus: the guess is at least the right genus. Skips count as questions, as they do on the site.

| games | n | right | genus | top 3 | questions (mean / worst) | skips | unsure | gave up |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| held-out | 0 | | | | | | | |
| held-out, label check passed | 0 | | | | | | | |
| held-out, clouds with 4+ photos | 0 | | | | | | | |
| tune | 8 | 25% | 62% | 38% | 15.1 / 20 | 4.1 | 0% | 25% |
| tune, label check passed | 6 | 33% | 83% | 50% | 15.2 / 20 | 3.8 | 0% | 33% |
| tune, clouds with 4+ photos | 7 | 29% | 71% | 43% | 14.9 / 20 | 3.7 | 0% | 29% |
| all | 8 | 25% | 62% | 38% | 15.1 / 20 | 4.1 | 0% | 25% |

## Questions

Agreement: answers that include the knowledge base's value for the true cloud, out of those answered. Skip causes: not in photo / ambiguous / unclear question. Asked: share of games that asked it. Gain: bits removed per answered ask, on average.

| question | answers | agreement | skipped | not in photo / ambiguous / unclear | low confidence | asked | gain |
| --- | --- | --- | --- | --- | --- | --- | --- |
| What shape was it? | 8 | 50% | 0% | 0 / 0 / 0 | 12% | 100% | 0.77 |
| How big was one lump, at arm's length? | 8 | 62% | 0% | 0 / 0 / 0 | 25% | 88% | 0.25 |
| Were the lumps shaded grey underneath? | 8 | 62% | 0% | 0 / 0 / 0 | 12% | 100% | 0.36 |
| Could you see the sun through it? | 8 | 100% | 75% | 6 / 0 / 0 | 0% | 100% | 0.41 |
| What colour was it? | 8 | 88% | 0% | 0 / 0 / 0 | 0% | 88% | 0.18 |
| How much of the sky did it cover? | 8 | 88% | 0% | 0 / 0 / 0 | 0% | 100% | 0.28 |
| What was its underside like? | 8 | 62% | 0% | 0 / 0 / 0 | 25% | 100% | -0.13 |
| Was anything falling from it? | 8 | 100% | 0% | 0 / 0 / 0 | 0% | 100% | 0.16 |
| Was there a ring of light around the sun or moon? | 8 | 100% | 88% | 7 / 0 / 0 | 0% | 75% | 0.15 |
| Did its top spread out flat, like an anvil? | 8 | 100% | 0% | 0 / 0 / 0 | 0% | 25% | -0.03 |
| Did little turrets rise from its top? | 8 | 62% | 0% | 0 / 0 / 0 | 12% | 75% | 0.12 |
| Were the streaks hooked, like commas? | 8 | 88% | 0% | 0 / 0 / 0 | 0% | 75% | 0.19 |
| Did it have rounded pouches hanging underneath? | 8 | 100% | 0% | 0 / 0 / 0 | 0% | 62% | -0.02 |
| How tall was it, next to how wide? | 8 | 75% | 0% | 0 / 0 / 0 | 0% | 62% | 0.23 |
| What was the top of it like? | 8 | 100% | 0% | 0 / 0 / 0 | 25% | 88% | -0.07 |

## Leakage check: things a photo cannot show

Whether rain was falling, or a halo was round the sun when the sun is out of frame, can rarely be judged from a photo. Confident answers that match the knowledge base here suggest the answerer is recalling the cloud type instead of looking.

| question | answered | answered with high confidence | agreement when answered |
| --- | --- | --- | --- |
| Was anything falling from it? | 100% | 38% | 100% |
| Was there a ring of light around the sun or moon? | 12% | 0% | 100% |

## Leakage check: recognised or not

In a separate call the answerer named 2 of 8 photos exactly. If its answers agree with the knowledge base far more often on those than on the rest, it is answering from what it knows about the type. Some gap is expected either way: a photo that is easy to name is often easy to answer.

| question | agreement, named right | agreement, named wrong |
| --- | --- | --- |
| What shape was it? | 100% of 2 | 33% of 6 |
| How big was one lump, at arm's length? | 50% of 2 | 67% of 6 |
| Were the lumps shaded grey underneath? | 50% of 2 | 67% of 6 |
| Could you see the sun through it? | - of 0 | 100% of 2 |
| What colour was it? | 50% of 2 | 100% of 6 |
| How much of the sky did it cover? | 100% of 2 | 83% of 6 |
| What was its underside like? | 100% of 2 | 50% of 6 |
| Was anything falling from it? | 100% of 2 | 100% of 6 |
| Was there a ring of light around the sun or moon? | - of 0 | 100% of 1 |
| Did its top spread out flat, like an anvil? | 100% of 2 | 100% of 6 |
| Did little turrets rise from its top? | 100% of 2 | 50% of 6 |
| Were the streaks hooked, like commas? | 100% of 2 | 83% of 6 |
| Did it have rounded pouches hanging underneath? | 100% of 2 | 100% of 6 |
| How tall was it, next to how wide? | 50% of 2 | 83% of 6 |
| What was the top of it like? | 100% of 2 | 100% of 6 |

## Clouds

| cloud | photos | games | right | questions | most common wrong guess |
| --- | --- | --- | --- | --- | --- |
| Cirrus fibratus | 8 | 1 | 0% | 14.0 | Cirrus uncinus (1) |
| Cirrus uncinus | 8 | 0 | - | - |  |
| Cirrus spissatus | 8 | 0 | - | - |  |
| Cirrus castellanus | 4 | 0 | - | - |  |
| Cirrus floccus | 8 | 1 | 0% | 11.0 | Cirrus fibratus (1) |
| Cirrocumulus stratiformis | 8 | 0 | - | - |  |
| Cirrocumulus lenticularis | 8 | 0 | - | - |  |
| Cirrocumulus floccus | 8 | 0 | - | - |  |
| Cirrocumulus castellanus | 1 | 1 | 0% | 17.0 | Cirrus fibratus (1) |
| Cirrostratus fibratus | 8 | 0 | - | - |  |
| Cirrostratus nebulosus | 8 | 0 | - | - |  |
| Altocumulus stratiformis | 8 | 0 | - | - |  |
| Altocumulus lenticularis | 8 | 1 | 100% | 20.0 |  |
| Altocumulus castellanus | 8 | 0 | - | - |  |
| Altocumulus floccus | 8 | 0 | - | - |  |
| Altocumulus volutus | 3 | 0 | - | - |  |
| Altostratus translucidus | 8 | 1 | 100% | 14.0 |  |
| Altostratus opacus | 8 | 0 | - | - |  |
| Stratocumulus stratiformis | 8 | 0 | - | - |  |
| Stratocumulus lenticularis | 8 | 0 | - | - |  |
| Stratocumulus castellanus | 8 | 1 | 0% | 13.0 | Stratocumulus stratiformis (1) |
| Stratocumulus floccus | 8 | 0 | - | - |  |
| Stratocumulus volutus | 3 | 0 | - | - |  |
| Stratus nebulosus | 8 | 0 | - | - |  |
| Stratus fractus | 8 | 1 | 0% | 13.0 | Stratocumulus stratiformis (1) |
| Cumulus humilis | 8 | 0 | - | - |  |
| Cumulus mediocris | 8 | 0 | - | - |  |
| Cumulus congestus | 8 | 0 | - | - |  |
| Cumulus fractus | 8 | 1 | 0% | 19.0 | Stratus fractus (1) |
| Cumulonimbus calvus | 8 | 0 | - | - |  |
| Cumulonimbus capillatus | 8 | 0 | - | - |  |
| Nimbostratus | 8 | 0 | - | - |  |

## Mix-ups

Rows: the knowledge base's value for the true cloud. Columns: what was answered. An answer torn between two options counts half to each.

### What shape was it?

| true \ answered | wispy | sheet | rolls | tufts | lens | roll_cloud | heaped | towering | flat_patches | ragged | (skip) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| wispy | 0.5 | 0.5 |  |  |  |  |  |  |  |  |  |
| sheet |  | 0.5 |  |  |  |  |  |  |  | 0.5 |  |
| tufts | 1.5 |  | 0.5 |  |  |  | 0.5 |  |  | 0.5 |  |
| lens |  |  |  |  | 1 |  |  |  |  |  |  |
| ragged |  |  | 1 | 0.5 |  |  |  |  |  | 0.5 |  |

### How big was one lump, at arm's length?

| true \ answered | none | tiny | small | large | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 3 |  | 1.5 | 0.5 |  |
| tiny | 1 |  |  |  |  |
| small | 1 |  |  |  |  |
| large |  |  | 0.5 | 0.5 |  |

### Were the lumps shaded grey underneath?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes | 2 | 1 |  |
| no | 2 | 3 |  |

### Could you see the sun through it?

| true \ answered | transparent | translucent | opaque | (skip) |
| --- | --- | --- | --- | --- |
| transparent | 1 |  |  | 2 |
| translucent |  | 1 |  | 1 |
| opaque |  |  |  | 3 |

### What colour was it?

| true \ answered | white | grey | dark_grey | (skip) |
| --- | --- | --- | --- | --- |
| white | 4 |  |  |  |
| grey | 1 | 3 |  |  |

### How much of the sky did it cover?

| true \ answered | isolated | patches | most_of_sky | whole_sky | (skip) |
| --- | --- | --- | --- | --- | --- |
| isolated | 1.5 | 0.5 |  |  |  |
| patches | 1 | 2 | 1 |  |  |
| most_of_sky |  | 1 |  |  |  |
| whole_sky |  |  |  | 1 |  |

### What was its underside like?

| true \ answered | flat | ragged | undefined | not_visible | (skip) |
| --- | --- | --- | --- | --- | --- |
| flat |  | 0.5 | 0.5 |  |  |
| ragged |  | 1 | 1.5 | 0.5 |  |
| undefined |  |  | 2 | 1 |  |
| not_visible |  | 0.5 | 0.5 |  |  |

### Was anything falling from it?

| true \ answered | none | virga | steady | showers | thunder | (skip) |
| --- | --- | --- | --- | --- | --- | --- |
| none | 6.5 | 0.5 |  |  |  |  |
| virga |  | 1 |  |  |  |  |

### Was there a ring of light around the sun or moon?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| no |  | 1 | 7 |

### Did its top spread out flat, like an anvil?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| no |  | 8 |  |

### Did little turrets rise from its top?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes |  | 2 |  |
| no | 1 | 5 |  |

### Were the streaks hooked, like commas?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| no | 1 | 7 |  |

### Did it have rounded pouches hanging underneath?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| no |  | 8 |  |

### How tall was it, next to how wide?

| true \ answered | none | flat | square | tall | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 3 | 5 |  |  |  |

### What was the top of it like?

| true \ answered | none | cauliflower | smooth | fibrous | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 6.5 |  | 1 | 0.5 |  |

## Games that went wrong

- `fdac68aa` Stratus fractus → Stratocumulus stratiformis (true cloud ranked #23, 13 questions): shape, opacity?, sky_cover, shading, mamma, element_size, opacity?, base, turrets, colour, opacity?, precipitation, top
- `d800bf7a` Cirrocumulus castellanus → Cirrus fibratus (true cloud ranked #13, 17 questions): shape, sky_cover, precipitation, opacity?, hooks, shading, halo?, opacity?, base, colour, halo?, opacity?, top, element_size, halo?, opacity?, depth
- `d443bae1` Stratocumulus castellanus → Stratocumulus stratiformis (true cloud ranked #13, 13 questions): shape, precipitation, sky_cover, depth, top, opacity?, element_size, mamma, shading, turrets, opacity?, base, colour
- `ff5e20e2` Cirrus floccus → Cirrus fibratus (true cloud ranked #7, 11 questions): shape, sky_cover, opacity, precipitation, hooks, shading, halo?, base, turrets, element_size, halo?
- `fe72556d` Cumulus fractus → Stratus fractus (true cloud ranked #5, 19 questions): shape, sky_cover, depth, top, base, shading, opacity?, precipitation, turrets, colour, opacity?, element_size, halo?, mamma, opacity?, anvil, halo?, hooks, opacity?
- `7c8a6472` Cirrus fibratus → Cirrus uncinus (true cloud ranked #2, 14 questions): shape, sky_cover, precipitation, opacity?, halo?, base, colour, opacity?, halo?, hooks, shading, opacity?, halo?, top
