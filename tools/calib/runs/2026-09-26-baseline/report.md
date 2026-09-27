# clouds: baseline (photo mode, claude-opus-5)

> Incomplete: 87 of 235 photos not asked yet, and 0 answers missing on the rest (they replay as gaps). Run again to fill them before trusting these numbers.

## Headline

Right: the true cloud ranked first. Genus: the guess is at least the right genus. Skips count as questions, as they do on the site.

| games | n | right | genus | top 3 | questions (mean / worst) | skips | unsure | gave up |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| held-out | 55 | 22% | 47% | 47% | 13.5 / 23 | 4.0 | 2% | 9% |
| held-out, label check passed | 35 | 31% | 57% | 66% | 13.2 / 23 | 3.8 | 0% | 11% |
| held-out, clouds with 4+ photos | 54 | 22% | 48% | 48% | 13.5 / 23 | 4.0 | 2% | 9% |
| tune | 93 | 23% | 41% | 37% | 15.3 / 32 | 5.5 | 10% | 17% |
| tune, label check passed | 56 | 34% | 50% | 52% | 14.6 / 23 | 5.0 | 7% | 7% |
| tune, clouds with 4+ photos | 90 | 23% | 42% | 38% | 15.3 / 32 | 5.5 | 10% | 18% |
| all | 148 | 22% | 43% | 41% | 14.6 / 32 | 5.0 | 7% | 14% |

## Questions

Agreement: answers that include the knowledge base's value for the true cloud, out of those answered. Skip causes: not in photo / ambiguous / unclear question. Asked: share of games that asked it. Gain: bits removed per answered ask, on average.

| question | answers | agreement | skipped | not in photo / ambiguous / unclear | low confidence | asked | gain |
| --- | --- | --- | --- | --- | --- | --- | --- |
| What shape was it? | 148 | 63% | 0% | 0 / 0 / 0 | 5% | 100% | 0.88 |
| How big was one lump, at arm's length? | 148 | 66% | 1% | 0 / 1 / 0 | 26% | 71% | 0.25 |
| Were the lumps shaded grey underneath? | 148 | 77% | 3% | 4 / 1 / 0 | 26% | 100% | 0.31 |
| Could you see the sun through it? | 148 | 47% | 74% | 110 / 0 / 0 | 13% | 98% | 0.18 |
| What colour was it? | 148 | 80% | 0% | 0 / 0 / 0 | 1% | 78% | 0.09 |
| How much of the sky did it cover? | 148 | 63% | 1% | 0 / 1 / 0 | 5% | 99% | 0.30 |
| What was its underside like? | 148 | 60% | 1% | 1 / 0 / 0 | 22% | 95% | 0.08 |
| Was anything falling from it? | 148 | 83% | 2% | 2 / 1 / 0 | 10% | 99% | 0.16 |
| Was there a ring of light around the sun or moon? | 148 | 76% | 80% | 118 / 1 / 0 | 34% | 88% | 0.34 |
| Did its top spread out flat, like an anvil? | 148 | 97% | 0% | 0 / 0 / 0 | 1% | 20% | -0.00 |
| Did little turrets rise from its top? | 148 | 92% | 1% | 1 / 1 / 0 | 23% | 57% | 0.19 |
| Were the streaks hooked, like commas? | 148 | 94% | 0% | 0 / 0 / 0 | 28% | 59% | 0.16 |
| Did it have rounded pouches hanging underneath? | 148 | 95% | 1% | 1 / 1 / 0 | 5% | 56% | 0.01 |
| How tall was it, next to how wide? | 148 | 74% | 1% | 1 / 0 / 0 | 7% | 34% | 0.56 |
| What was the top of it like? | 148 | 90% | 1% | 1 / 0 / 0 | 24% | 65% | 0.06 |

## Leakage check: things a photo cannot show

Whether rain was falling, or a halo was round the sun when the sun is out of frame, can rarely be judged from a photo. Confident answers that match the knowledge base here suggest the answerer is recalling the cloud type instead of looking.

| question | answered | answered with high confidence | agreement when answered |
| --- | --- | --- | --- |
| Was anything falling from it? | 98% | 38% | 83% |
| Was there a ring of light around the sun or moon? | 20% | 7% | 76% |

## Leakage check: recognised or not

In a separate call the answerer named 3 of 12 photos exactly. If its answers agree with the knowledge base far more often on those than on the rest, it is answering from what it knows about the type. Some gap is expected either way: a photo that is easy to name is often easy to answer.

| question | agreement, named right | agreement, named wrong |
| --- | --- | --- |
| What shape was it? | 100% of 3 | 56% of 9 |
| How big was one lump, at arm's length? | 0% of 3 | 67% of 9 |
| Were the lumps shaded grey underneath? | 33% of 3 | 88% of 8 |
| Could you see the sun through it? | - of 0 | 100% of 1 |
| What colour was it? | 67% of 3 | 100% of 9 |
| How much of the sky did it cover? | 67% of 3 | 78% of 9 |
| What was its underside like? | 67% of 3 | 67% of 9 |
| Was anything falling from it? | 100% of 3 | 89% of 9 |
| Was there a ring of light around the sun or moon? | - of 0 | 100% of 1 |
| Did its top spread out flat, like an anvil? | 100% of 3 | 100% of 9 |
| Did little turrets rise from its top? | 100% of 3 | 88% of 8 |
| Were the streaks hooked, like commas? | 100% of 3 | 89% of 9 |
| Did it have rounded pouches hanging underneath? | 100% of 3 | 100% of 9 |
| How tall was it, next to how wide? | 33% of 3 | 78% of 9 |
| What was the top of it like? | 100% of 3 | 89% of 9 |

## Clouds

| cloud | photos | games | right | questions | most common wrong guess |
| --- | --- | --- | --- | --- | --- |
| Cirrus fibratus | 8 | 8 | 62% | 13.0 | Cirrus uncinus (3) |
| Cirrus uncinus | 8 | 8 | 62% | 11.0 | Cirrus fibratus (3) |
| Cirrus spissatus | 8 | 8 | 0% | 14.2 | Cirrus fibratus (3) |
| Cirrus castellanus | 4 | 4 | 0% | 15.5 | Cirrus fibratus (2) |
| Cirrus floccus | 8 | 8 | 0% | 12.2 | Cirrus fibratus (5) |
| Cirrocumulus stratiformis | 8 | 8 | 88% | 17.4 | Cirrus fibratus (1) |
| Cirrocumulus lenticularis | 8 | 8 | 0% | 16.6 | Cirrus spissatus (3) |
| Cirrocumulus floccus | 8 | 8 | 0% | 18.1 | Cirrocumulus stratiformis (5) |
| Cirrocumulus castellanus | 1 | 1 | 0% | 9.0 | Cirrus fibratus (1) |
| Cirrostratus fibratus | 8 | 8 | 0% | 11.9 | Cirrus fibratus (6) |
| Cirrostratus nebulosus | 8 | 8 | 38% | 10.2 | Altostratus translucidus (3) |
| Altocumulus stratiformis | 8 | 8 | 0% | 15.1 | Stratocumulus stratiformis (4) |
| Altocumulus lenticularis | 8 | 8 | 12% | 18.5 | Cumulus humilis (3) |
| Altocumulus castellanus | 8 | 8 | 12% | 18.4 | Stratocumulus stratiformis (3) |
| Altocumulus floccus | 8 | 8 | 0% | 17.0 | Stratocumulus stratiformis (3) |
| Altocumulus volutus | 3 | 3 | 0% | 18.0 | Stratus nebulosus (1) |
| Altostratus translucidus | 8 | 8 | 62% | 10.9 | Stratus nebulosus (2) |
| Altostratus opacus | 8 | 8 | 0% | 15.2 | Stratus nebulosus (3) |
| Stratocumulus stratiformis | 8 | 8 | 75% | 12.1 | Stratocumulus floccus (1) |
| Stratocumulus lenticularis | 8 | 8 | 0% | 15.6 | Stratocumulus stratiformis (2) |
| Stratocumulus castellanus | 8 | 2 | 0% | 13.5 | Cumulus humilis (2) |
| Stratocumulus floccus | 8 | 0 | - | - |  |
| Stratocumulus volutus | 3 | 0 | - | - |  |
| Stratus nebulosus | 8 | 0 | - | - |  |
| Stratus fractus | 8 | 1 | 0% | 13.0 | Stratocumulus stratiformis (1) |
| Cumulus humilis | 8 | 0 | - | - |  |
| Cumulus mediocris | 8 | 0 | - | - |  |
| Cumulus congestus | 8 | 0 | - | - |  |
| Cumulus fractus | 8 | 1 | 0% | 20.0 | Something not in this guide (1) |
| Cumulonimbus calvus | 8 | 0 | - | - |  |
| Cumulonimbus capillatus | 8 | 0 | - | - |  |
| Nimbostratus | 8 | 0 | - | - |  |

## Mix-ups

Rows: the knowledge base's value for the true cloud. Columns: what was answered. An answer torn between two options counts half to each.

### What shape was it?

| true \ answered | wispy | sheet | rolls | tufts | lens | roll_cloud | heaped | towering | flat_patches | ragged | (skip) |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| wispy | 21.5 | 1.5 |  | 0.5 |  |  |  |  |  | 0.5 |  |
| sheet | 2 | 16.5 | 2.5 |  |  | 0.5 | 0.5 |  | 1 | 1 |  |
| rolls | 2 | 1.5 | 18.5 | 0.5 | 0.5 |  | 0.5 |  | 0.5 |  |  |
| tufts | 11 | 1.5 | 17.5 | 5 |  |  | 3 |  |  | 1 |  |
| lens | 3 |  | 4 | 0.5 | 12.5 | 0.5 | 2 |  | 1 | 0.5 |  |
| roll_cloud |  | 2 | 0.5 |  |  |  |  |  | 0.5 |  |  |
| flat_patches | 3.5 | 0.5 |  |  | 0.5 |  | 1.5 | 0.5 | 1.5 |  |  |
| ragged |  |  | 1 | 0.5 |  |  |  |  |  | 0.5 |  |

### How big was one lump, at arm's length?

| true \ answered | none | tiny | small | large | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 52.5 | 5 | 8 | 4.5 |  |
| tiny | 6 | 14.5 | 2.5 | 1 | 1 |
| small | 6 | 6.5 | 16.5 | 6 |  |
| large | 5 |  | 1.5 | 11.5 |  |

### Were the lumps shaded grey underneath?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes | 33 | 20 | 1 |
| no | 13 | 77 | 4 |

### Could you see the sun through it?

| true \ answered | transparent | translucent | opaque | (skip) |
| --- | --- | --- | --- | --- |
| transparent | 1.5 | 13.5 |  | 54 |
| translucent |  | 8.5 | 2.5 | 32 |
| opaque | 1 | 4 | 7 | 24 |

### What colour was it?

| true \ answered | white | grey | dark_grey | (skip) |
| --- | --- | --- | --- | --- |
| white | 87 | 16 | 1 |  |
| grey | 16.5 | 18.5 | 1 |  |
| dark_grey |  | 7.5 | 0.5 |  |

### How much of the sky did it cover?

| true \ answered | isolated | patches | most_of_sky | whole_sky | (skip) |
| --- | --- | --- | --- | --- | --- |
| isolated | 4 | 10 | 4.5 | 1.5 |  |
| patches | 5 | 44 | 17.5 | 2.5 | 1 |
| most_of_sky |  | 5.5 | 17.5 | 3 |  |
| whole_sky | 1 | 5.5 | 6.5 | 19 |  |

### What was its underside like?

| true \ answered | flat | ragged | undefined | not_visible | (skip) |
| --- | --- | --- | --- | --- | --- |
| flat | 3.5 | 1 | 11 | 2.5 |  |
| ragged |  | 0.5 | 22.5 | 3 |  |
| undefined | 4 | 3.5 | 67.5 | 12 | 1 |
| not_visible |  | 1 | 8 | 7 |  |

### Was anything falling from it?

| true \ answered | none | virga | steady | showers | thunder | (skip) |
| --- | --- | --- | --- | --- | --- | --- |
| none | 111 | 2 |  |  |  | 3 |
| virga | 26.5 | 5.5 |  |  |  |  |

### Was there a ring of light around the sun or moon?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes | 4 | 7 | 5 |
| no |  | 18 | 114 |

### Did its top spread out flat, like an anvil?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| no | 4 | 144 |  |

### Did little turrets rise from its top?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes | 4 | 10 | 1 |
| no | 2 | 130 | 1 |

### Were the streaks hooked, like commas?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes | 5.5 | 2.5 |  |
| no | 7 | 133 |  |

### Did it have rounded pouches hanging underneath?

| true \ answered | yes | no | (skip) |
| --- | --- | --- | --- |
| yes |  | 8 |  |
| no |  | 138 | 2 |

### How tall was it, next to how wide?

| true \ answered | none | flat | square | tall | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 99.5 | 46.5 | 1 |  | 1 |

### What was the top of it like?

| true \ answered | none | cauliflower | smooth | fibrous | (skip) |
| --- | --- | --- | --- | --- | --- |
| none | 128 | 4 | 8 | 7 | 1 |

## Games that went wrong

- `26af4a3f` Altocumulus volutus → Stratus nebulosus (true cloud ranked #28, 14 questions): shape, precipitation, sky_cover, halo?, opacity?, colour, base, halo?, opacity?, shading, element_size, halo?, opacity?, hooks
- `7411771e` Cirrocumulus lenticularis → Cumulus humilis (true cloud ranked #32, 5 questions): shape, precipitation, depth, top, shading
- `2a506591` Cirrus castellanus → Cumulus humilis (true cloud ranked #26, 13 questions): shape, precipitation, sky_cover, depth, top, element_size, opacity?, mamma, base, colour, opacity?, shading, turrets
- `f7a8bff5` Altocumulus castellanus → Cirrus fibratus (true cloud ranked #27, 25 questions): shape, sky_cover, precipitation, opacity?, shading, halo?, hooks, opacity?, base, halo?, colour, opacity?, top, halo?, depth, opacity?, element_size, halo?, anvil, opacity?, mamma, halo?, turrets, opacity?, halo?
- `2e332230` Altocumulus volutus → Cirrostratus nebulosus (true cloud ranked #28, 29 questions): shape, precipitation, sky_cover, halo?, opacity?, base, shading?, halo?, opacity?, colour, shading?, halo?, opacity?, element_size, shading?, halo?, opacity?, hooks, shading?, halo?, opacity?, mamma, shading?, halo?, opacity?, top, shading?, halo?, opacity?
- `480261ba` Cirrus spissatus → Cumulus mediocris (true cloud ranked #15, 16 questions): shape, precipitation, depth, sky_cover, top, anvil, mamma, colour, shading, halo?, base, opacity?, element_size, halo?, turrets, hooks
- `fdac68aa` Stratus fractus → Stratocumulus stratiformis (true cloud ranked #23, 13 questions): shape, opacity?, sky_cover, shading, mamma, element_size, opacity?, base, turrets, colour, opacity?, precipitation, top
- `36177d5c` Stratocumulus lenticularis → Cirrus fibratus (true cloud ranked #27, 9 questions): shape, sky_cover, precipitation, opacity?, hooks, shading, halo?, opacity?, base
- `574c9a5b` Stratocumulus castellanus → Cumulus humilis (true cloud ranked #12, 7 questions): shape, precipitation, sky_cover, opacity?, depth, top, shading
- `f36eff88` Altostratus opacus → Something not in this guide (true cloud ranked #26, 21 questions): shape, precipitation, sky_cover, depth, top, opacity?, element_size, base, shading, opacity?, colour, mamma, halo?, opacity?, turrets, anvil, halo?, opacity?, hooks, opacity?, halo?
- `29aaf127` Altostratus opacus → Stratocumulus stratiformis (true cloud ranked #23, 13 questions): shape, opacity?, sky_cover, shading, mamma, element_size, opacity?, base, turrets, colour, opacity?, precipitation, top
- `1f5f2c86` Cirrus floccus → Cirrus uncinus (true cloud ranked #19, 12 questions): shape, sky_cover, precipitation, opacity?, hooks, shading, halo?, opacity?, top, base, halo?, opacity?
- `fdaead43` Stratocumulus lenticularis → Cirrus fibratus (true cloud ranked #22, 17 questions): shape, sky_cover, opacity?, precipitation, shading, hooks, opacity?, halo?, colour, element_size, opacity?, halo?, base, top, opacity?, halo?, turrets
- `60e28bc9` Cirrus floccus → Cirrus fibratus (true cloud ranked #18, 11 questions): shape, sky_cover, precipitation, halo, opacity, shading, colour, base, hooks, element_size, mamma
- `ea1a2712` Cirrus spissatus → Cumulus humilis (true cloud ranked #22, 14 questions): shape, precipitation, sky_cover, depth, top, opacity?, shading, base, element_size, opacity?, turrets, mamma, halo?, opacity?
- `b23b0d2c` Stratocumulus lenticularis → Stratus nebulosus (true cloud ranked #16, 15 questions): shape, sky_cover, precipitation?, opacity, shading, base, precipitation?, colour, element_size, halo?, precipitation?, top, depth, halo?, precipitation?
- `860e9d01` Cirrus castellanus → Stratocumulus stratiformis (true cloud ranked #20, 17 questions): shape, sky_cover, opacity?, shading, mamma, element_size, base, opacity?, turrets?, colour, precipitation, opacity?, turrets?, halo?, top, opacity?, turrets?
- `03010414` Cirrus spissatus → Cumulus humilis (true cloud ranked #14, 6 questions): shape, precipitation, sky_cover, depth, top, shading
- `90ed8f2a` Cirrus floccus → Cirrus uncinus (true cloud ranked #15, 9 questions): shape, sky_cover, precipitation, hooks, opacity?, shading, halo?, top, opacity?
- `1d6f5dba` Cirrus floccus → Cirrus fibratus (true cloud ranked #21, 9 questions): shape, sky_cover, precipitation, opacity?, hooks, shading, halo?, opacity?, base

## Calibration

Measured on the tune split and blended with the current values as 10 observations (see calibrate.py). Measured rates are in brackets.

| question | answered | noise | confusion | cost | look-alikes added | declared, never seen |
| --- | --- | --- | --- | --- | --- | --- |
| What shape was it? | 93 of 93 | 0.10 → 0.14 (0.14) | 0.35 → 0.38 (0.39) | 1.0 → 1.1 | flat_patches/wispy, tufts/wispy | heaped/towering |
| How big was one lump, at arm's length? | 92 of 93 | 0.12 → 0.02 (0.01) | 0.40 → 0.38 (0.38) | 1.8 → 1.3 | large/none, none/small |  |
| Were the lumps shaded grey underneath? | 88 of 93 | 0.12 → 0.21 (0.22) | - | 1.8 → 1.4 |  |  |
| Could you see the sun through it? | 24 of 93 | 0.10 → 0.06 (0.04) | 0.35 → 0.44 (0.48) | 0.8 → 2.6 |  |  |
| What colour was it? | 93 of 93 | 0.12 → 0.02 (0.01) | 0.40 → 0.30 (0.29) | 0.7 → 1.0 |  |  |
| How much of the sky did it cover? | 92 of 93 | 0.12 → 0.02 (0.01) | 0.40 → 0.46 (0.47) | 0.7 → 1.1 | isolated/most_of_sky, patches/whole_sky |  |
| What was its underside like? | 92 of 93 | 0.12 → 0.07 (0.07) | 0.35 → 0.42 (0.42) | 1.4 → 1.3 |  |  |
| Was anything falling from it? | 90 of 93 | 0.08 → 0.02 (0.00) | 0.30 → 0.21 (0.20) | 0.8 → 1.2 |  | showers/steady, showers/thunder |
| Was there a ring of light around the sun or moon? | 18 of 93 | 0.06 → 0.20 (0.28) | - | 1.6 → 2.8 |  |  |
| Did its top spread out flat, like an anvil? | 93 of 93 | 0.07 → 0.03 (0.02) | - | 1.6 → 1.0 |  |  |
| Did little turrets rise from its top? | 91 of 93 | 0.10 → 0.09 (0.09) | - | 2.5 → 1.3 |  |  |
| Were the streaks hooked, like commas? | 93 of 93 | 0.10 → 0.05 (0.04) | - | 2.5 → 1.3 |  |  |
| Did it have rounded pouches hanging underneath? | 92 of 93 | 0.08 → 0.06 (0.05) | - | 2.2 → 1.1 |  |  |
| How tall was it, next to how wide? | 92 of 93 | 0.12 → 0.02 (0.00) | 0.25 → 0.30 (0.30) | 1.8 → 1.1 | flat/none | flat/square, square/tall |
| What was the top of it like? | 92 of 93 | 0.10 → 0.11 (0.11) | - | 2.0 → 1.3 |  | cauliflower/smooth, fibrous/smooth |

## Calibrated against current, held-out photos

| | current | calibrated |
| --- | --- | --- |
| games | 55 | 55 |
| right | 22% | 22% |
| questions | 13.5 | 12.5 |
| score (right - 5 x questions) | -45.5 | -40.5 |

Right under the candidate only: 0. Right under the current one only: 0 (at most 2 allowed).

**Accepted.**
