# GOLD-lite review — en→de (German)

Set: `bench-quality/gold/en-de.jsonl` — 300 sentences, *The Adventures of Sherlock Holmes*, chapters 1–4.

Reference translations: **google/gemini-3.7-flash**, no repair pass. Adversarial check: **claude sonnet (via the claude CLI)**, one verdict per sentence.

Checker verdicts: **wrong 6**, **minor 40**, **ok 254**.

## How to use this file

**No one on the team reads German.** This file exists for provenance and for a
future reader; nothing in it is promoted to `gold` by this pass. Every item keeps
the machine verdict: `silver` where the adversarial checker said `ok`,
`disputed` where it said `minor` or `wrong`. Treat the whole set as **silver**:
usable as an absolute regression alarm, never as a ground truth.

## Items

### Disputed — checker verdict `wrong`

**1. `h4-023`** · ch2 ¶34.0 · `wrong` · status `disputed`

- EN: As he spoke there was the sharp sound of horses’ hoofs and grating wheels against the curb, followed by a sharp pull at the bell.
- DE: Während er sprach, ertönte das scharfe Geräusch von Pferdehufe und knirschenden Rädern am Bordstein, gefolgt von einem scharfen Zug an der Klingel.
- checker: «von Pferdehufe» is wrong case after «von» (dative plural), should be «Pferdehufen»

**2. `h4-055`** · ch2 ¶129.0 · `wrong` · status `disputed`

- EN: “I then lounged down the street and found, as I expected, that there was a mews in a lane which runs down by one wall of the garden.
- DE: „Ich schlenderte dann die Straße hinunter und stellte fest, wie ich es erwartet hatte, dass sich in einer Gasse, die an einer Mauer des Gartens entlangläuft, Mews befanden.
- checker: «Mews» left untranslated as an English word standing in as a German plural noun

**3. `h4-072`** · ch2 ¶159.1 · `wrong` · status `disputed`

- EN: There is only one point on which I must insist.
- DE: Es gibt nur einen Punkt, auf den ich bestehen muss.
- checker: «bestehen auf den» — «bestehen auf» governs Dativ, not Akkusativ; should be «auf dem»

**4. `h4-083`** · ch2 ¶182.0 · `wrong` · status `disputed`

- EN: “I will not look.”
- DE: „Ich werde nicht suchen.“
- checker: «suchen» means 'search', not 'look' — changes the meaning of the source sentence

**5. `h4-207`** · ch3 ¶187.1 · `wrong` · status `disputed`

- EN: “That is back through the house into Saxe-Coburg Square.
- DE: „Der führt zurück durch das Haus auf den Sachsen-Coburg Square.
- checker: proper name distorted: «Sachsen-Coburg Square» half-translates 'Saxe-Coburg Square'

**6. `h4-230`** · ch4 ¶4.8 · `wrong` · status `disputed`

- EN: The crudest of writers could invent nothing more crude.”
- DE: Der stümperhafteste aller Schriftsteller könnte nichts Plumpers erfinden.“
- checker: «nichts Plumpers» has a bogus ending; comparative nominalized after nichts must be «nichts Plumperes»

### Disputed — checker verdict `minor`

**7. `h4-002`** · ch2 ¶1.1 · `minor` · status `disputed`

- EN: I have seldom heard him mention her under any other name.
- DE: Ich habe ihn selten ihren Namen unter einer anderen Bezeichnung erwähnen hören.
- checker: convoluted rendering of «mention her under any other name» as «ihren Namen unter einer anderen Bezeichnung erwähnen», redundantly pairing «Namen» with «Bezeichnung»

**8. `h4-009`** · ch2 ¶8.0 · `minor` · status `disputed`

- EN: “Then, how do you know?”
- DE: „Woher weißt du es dann?“
- checker: address shifts to informal «du» right after Holmes uses formal «Sie» to Watson in the same exchange (h4-008)

**9. `h4-022`** · ch2 ¶33.2 · `minor` · status `disputed`

- EN: Do you note the peculiar construction of the sentence—‘This account of you we have from all quarters received.
- DE: Bemerkst du den eigentümlichen Satzbau – ‚Diesen Bericht über Sie haben wir aus allen Richtungen erhalten‘?
- checker: the word order in «Diesen Bericht über Sie haben wir aus allen Richtungen erhalten» is normal German syntax, so it no longer reads as the 'peculiar construction' the source is pointing out

**10. `h4-029`** · ch2 ¶45.0 · `minor` · status `disputed`

- EN: “You may address me as the Count Von Kramm, a Bohemian nobleman.
- DE: „Sie können mich als Graf von Kramm anreden, einen böhmischen Adligen.
- checker: the apposition «einen böhmischen Adligen» is accusative while «als Graf von Kramm» is nominative, making the case agreement of the apposition ambiguous/inconsistent

**11. `h4-034`** · ch2 ¶57.1 · `minor` · status `disputed`

- EN: “Your Majesty had not spoken before I was aware that I was addressing Wilhelm Gottsreich Sigismond von Ormstein, Grand Duke of Cassel-Felstein, and hereditary King of Bohemia.”
- DE: „Eure Majestät hatten noch nicht gesprochen, ehe mir bewusst war, dass ich mich an Wilhelm Gottsreich Sigismond von Ormstein, Großherzog von Cassel-Felstein und Erbkönig von Böhmen, wandte.“
- checker: «Eure Majestät hatten» uses plural verb agreement with a singular address form, inconsistent with the singular «hat»/«muss» used elsewhere for the same title

**12. `h4-036`** · ch2 ¶62.0 · `minor` · status `disputed`

- EN: “Let me see!” said Holmes.
- DE: „Lassen Sie mich sehen!“, sagte Holmes.
- checker: «Lassen Sie mich sehen!» reads as literal visual «let me look» rather than the idiomatic pondering sense of «Let me see!»

**13. `h4-059`** · ch2 ¶132.6 · `minor` · status `disputed`

- EN: If the latter, it was less likely.
- DE: Wenn Letzteres, war es weniger wahrscheinlich.
- checker: «Wenn Letzteres, war es» is an unnatural elliptical calque of the English fragment, missing a verb in the conditional clause

**14. `h4-060`** · ch2 ¶134.0 · `minor` · status `disputed`

- EN: “I was still balancing the matter in my mind when a hansom cab drove up to Briony Lodge, and a gentleman sprang out.
- DE: „Ich wog die Angelegenheit noch in meinem Geist ab, als ein Hansom-Cab bei Briony Lodge vorfuhr und ein Gentleman heraussprang.
- checker: «in meinem Geist abwägen» is a slightly unidiomatic literal rendering of 'balancing in my mind', where German normally uses the fixed phrase «im Geiste»

**15. `h4-062`** · ch2 ¶136.1 · `minor` · status `disputed`

- EN: It hadn’t pulled up before she shot out of the hall door and into it.
- DE: Er hatte noch nicht gehalten, ehe sie aus der Haustür und hinein geschossen kam.
- checker: «aus der Haustür und hinein geschossen kam» is a contorted coordination, barely parseable

**16. `h4-077`** · ch2 ¶172.3 · `minor` · status `disputed`

- EN: His expression, his manner, his very soul seemed to vary with every fresh part that he assumed.
- DE: Sein Ausdruck, sein Verhalten, seine eigentliche Seele schienen sich mit jeder neuen Rolle, die er annahm, zu verändern.
- checker: «seine eigentliche Seele» shifts «very soul» (emphatic) to «actual/real soul» (contrastive)

**17. `h4-082`** · ch2 ¶178.7 · `minor` · status `disputed`

- EN: It must be where she can lay her hands upon it.
- DE: Es muss dort sein, wo sie ihre Hände darauf legen kann.
- checker: «ihre Hände darauf legen» is an awkward literal calque of the idiom, sounds unnatural in German

**18. `h4-096`** · ch2 ¶206.5 · `minor` · status `disputed`

- EN: They laid me on a couch, I motioned for air, they were compelled to open the window, and you had your chance.”
- DE: Sie legten mich auf eine Couch, ich deutete an, dass ich Luft brauchte, sie waren gezwungen, das Fenster zu öffnen, und Sie hatten Ihre Chance.“
- checker: «deutete an» reads as a hint/implication rather than the physical gesture of «motioned»

**19. `h4-112`** · ch2 ¶245.11 · `minor` · status `disputed`

- EN: I sent John, the coachman, to watch you, ran upstairs, got into my walking clothes, as I call them, and came down just as you departed.
- DE: Ich schickte John, den Kutscher, um Sie zu beobachten, rannte nach oben, zog meine Spazierkleider an, wie ich sie nenne, und kam gerade herunter, als Sie gingen.
- checker: «Spazierkleider» is an unusual, non-standard compound for 'walking clothes'

**20. `h4-123`** · ch3 ¶12.5 · `minor` · status `disputed`

- EN: I ask you not merely because my friend Dr. Watson has not heard the opening part but also because the peculiar nature of the story makes me anxious to have every possible detail from your lips.
- DE: Ich bitte Sie darum nicht nur, weil mein Freund Dr. Watson den Anfang nicht gehört hat, sondern auch, weil die eigentümliche Natur der Geschichte mich begierig macht, jede erdenkliche Einzelheit aus Ihren eigenen Lippen zu vernehmen.
- checker: adds unwarranted emphasis «Ihren eigenen Lippen» ('your own lips') not present in source's 'your lips'

**21. `h4-124`** · ch3 ¶14.0 · `minor` · status `disputed`

- EN: I did not gain very much, however, by my inspection.
- DE: Durch meine Musterung gewann ich jedoch nicht sonderlich viel.
- checker: «Musterung» normally denotes a military conscription exam, an odd/ambiguous fit for casual visual 'inspection' of a person

**22. `h4-138`** · ch3 ¶44.2 · `minor` · status `disputed`

- EN: If my hair would only change colour, here’s a nice little crib all ready for me to step into.
- DE: Wenn mein Haar nur die Farbe wechseln würde, wäre hier ein schönes kleines Plätzchen, ganz bereit für mich, um hineinzuschlüpfen.‹
- checker: «Plätzchen» primarily reads as "cookie" and only loosely conveys "crib" as a cushy job/berth

**23. `h4-140`** · ch3 ¶51.0 · `minor` · status `disputed`

- EN: “Well, you can easily think that that made me prick up my ears, for the business has not been over good for some years, and an extra couple of hundred would have been very handy.
- DE: Nun, Sie können sich leicht denken, dass mich das die Ohren spitzen ließ, denn das Geschäft lief seit einigen Jahren nicht übermäßig gut, und ein zusätzliches Paar Hundert wäre sehr gelegen gekommen.
- checker: «ein zusätzliches Paar Hundert» is awkward/non-idiomatic phrasing for "a couple of hundred" (should be lowercase "ein paar hundert")

**24. `h4-145`** · ch3 ¶59.0 · `minor` · status `disputed`

- EN: “There was nothing in the office but a couple of wooden chairs and a deal table, behind which sat a small man with a head that was even redder than mine.
- DE: „Es gab nichts im Büro außer ein paar Holzstühlen und einem Fichtentisch, hinter dem ein kleiner Mann mit einem Kopf saß, der noch röter war als meiner.
- checker: «Fichtentisch» renders «deal table» as spruce rather than the cheap pine/fir wood implied by «deal»

**25. `h4-162`** · ch3 ¶96.3 · `minor` · status `disputed`

- EN: But there is, if you will excuse my saying so, something just a little funny about it.
- DE: Aber es gibt, wenn Sie mir die Bemerkung verzeihen, etwas geradezu ein wenig Komisches daran.
- checker: «geradezu ein wenig Komisches» is a self-contradictory pairing of an intensifier with a diminisher

**26. `h4-176`** · ch3 ¶133.0 · `minor` · status `disputed`

- EN: “What are you going to do, then?”
- DE: „Was wirst du denn tun?“
- checker: informal «du» is an unmotivated register shift from the formal «Sie» used elsewhere in this exchange

**27. `h4-179`** · ch3 ¶138.1 · `minor` · status `disputed`

- EN: It was a poky, little, shabby-genteel place, where four lines of dingy two-storied brick houses looked out into a small railed-in enclosure, where a lawn of weedy grass and a few clumps of faded laurel bushes made a hard fight against a smoke-laden and uncongenial atmosphere.
- DE: Es war ein armseliger, kleiner, scheinbar vornehmer Ort, wo vier Reihen schmuddeliger zweistöckiger Backsteinhäuser auf eine kleine, eingezäunte Anlage blickten, in der ein Rasen aus unkrautartigem Gras und ein paar Büschel verblichener Lorbeersträucher einen harten Kampf gegen eine rauchgeschwängerte und unwirtliche Atmosphäre führten.
- checker: «Büschel Lorbeersträucher» is an awkward collocation for clumps of bushes

**28. `h4-181`** · ch3 ¶141.2 · `minor` · status `disputed`

- EN: I have known something of him before.”
- DE: Ich habe schon früher einiges über ihn erfahren.“
- checker: «habe ... erfahren» shifts "have known" (prior acquaintance) to "found out/learned", a small epistemic nuance change

**29. `h4-183`** · ch3 ¶149.2 · `minor` · status `disputed`

- EN: We know something of Saxe-Coburg Square.
- DE: Wir wissen etwas über den Sachsen-Coburg Square.
- checker: «den Sachsen-Coburg Square» is an inconsistent hybrid — the name half-translated (Saxe→Sachsen) while «Square» is left in English

**30. `h4-192`** · ch3 ¶165.1 · `minor` · status `disputed`

- EN: “Our friend here is a wonderful man for starting a chase.
- DE: „Unser Freund hier ist ein wundervoller Mann, um eine Jagd zu beginnen.
- checker: «ein wundervoller Mann, um eine Jagd zu beginnen» renders the idiom 'a wonderful man for starting a chase' (= good at it) as an awkward purpose clause

**31. `h4-202`** · ch3 ¶180.0 · `minor` · status `disputed`

- EN: “It is our French gold,” whispered the director.
- DE: „Es ist unser Französisches Gold“, flüsterte der Direktor.
- checker: adjective «Französisches» wrongly capitalized mid-sentence in German

**32. `h4-203`** · ch3 ¶182.4 · `minor` · status `disputed`

- EN: Our reserve of bullion is much larger at present than is usually kept in a single branch office, and the directors have had misgivings upon the subject.”
- DE: Unser Vorrat an Barren und Münzen ist gegenwärtig viel größer, als üblicherweise in einer einzelnen Zweigstelle aufbewahrt wird, und die Direktoren hatten diesbezüglich Bedenken.“
- checker: «Barren und Münzen» adds coins not present in source «bullion»

**33. `h4-205`** · ch3 ¶185.4 · `minor` · status `disputed`

- EN: These are daring men, and though we shall take them at a disadvantage, they may do us some harm unless we are careful.
- DE: Das sind verwegene Männer, und obwohl wir sie im Nachteil erwischen werden, könnten sie uns Schaden zufügen, wenn wir nicht vorsichtig sind.
- checker: awkward collocation «im Nachteil erwischen» for 'take them at a disadvantage'

**34. `h4-208`** · ch3 ¶190.0 · `minor` · status `disputed`

- EN: What a time it seemed!
- DE: Was für eine Zeitspanne schien es doch zu sein!
- checker: stiff, technical-sounding «Was für eine Zeitspanne schien es doch zu sein!»

**35. `h4-212`** · ch3 ¶195.0 · `minor` · status `disputed`

- EN: “It’s no use, John Clay,” said Holmes blandly.
- DE: „Es hat keinen Zweck, John Clay“, sagte Holmes verbindlich.
- checker: «verbindlich» skews toward courteous/obliging, narrower than the flat, unemotional «blandly»

**36. `h4-215`** · ch3 ¶202.0 · `minor` · status `disputed`

- EN: “All right,” said Jones with a stare and a snigger.
- DE: „In Ordnung“, sagte Jones mit einem Starren und einem Kichern.
- checker: «Kichern» reads as a plain giggle, losing the sly/mocking edge of «snigger»

**37. `h4-232`** · ch4 ¶9.0 · `minor` · status `disputed`

- EN: “It was from the reigning family of Holland, though the matter in which I served them was of such delicacy that I cannot confide it even to you, who have been good enough to chronicle one or two of my little problems.”
- DE: „Der stammte von der regierenden Familie von Holland, obwohl die Angelegenheit, in der ich ihnen diente, von solcher Zartheit war, dass ich sie nicht einmal Ihnen anvertrauen kann, der Sie so freundlich waren, ein oder zwei meiner kleinen Probleme zu berichten.“
- checker: «berichten» weakens «chronicle» (a settled record) to a generic verbal report

**38. `h4-234`** · ch4 ¶12.1 · `minor` · status `disputed`

- EN: Looking over his shoulder, I saw that on the pavement opposite there stood a large woman with a heavy fur boa round her neck, and a large curling red feather in a broad-brimmed hat which was tilted in a coquettish Duchess of Devonshire fashion over her ear.
- DE: Über seine Schulter blickend, sah ich, dass auf dem Bürgersteig gegenüber eine stattliche Frau mit einer schweren Pelzboa um den Hals stand und einer großen, sich kräuselnden roten Feder an einem breitkrempigen Hut, der auf kokette Herzogin-von-Devonshire-Art über ihr Ohr geneigt war.
- checker: «stattliche Frau» shifts «large woman» toward stately/imposing rather than simply big

**39. `h4-239`** · ch4 ¶20.1 · `minor` · status `disputed`

- EN: “Yes, I did bang out of the house,” she said, “for it made me angry to see the easy way in which Mr. Windibank—that is, my father—took it all.
- DE: „Ja, ich bin tatsächlich aus dem Haus gestürzt“, sagte sie, „denn es machte mich wütend zu sehen, wie leichtfertig Mr. Windibank – das heißt, mein Vater – das alles nahm.
- checker: «leichtfertig» (reckless) adds a moral judgment not in «the easy way»

**40. `h4-251`** · ch4 ¶49.1 · `minor` · status `disputed`

- EN: He said that if they were sent to the office he would be chaffed by all the other clerks about having letters from a lady, so I offered to typewrite them, like he did his, but he wouldn’t have that, for he said that when I wrote them they seemed to come from me, but when they were typewritten he always felt that the machine had come between us.
- DE: Er sagte, wenn sie ins Büro geschickt würden, würde er von all den anderen Angestellten damit aufgezogen werden, Briefe von einer Dame zu bekommen, also bot ich an, sie mit der Schreibmaschine zu schreiben, so wie er die seinen, aber das wollte er nicht, denn er sagte, wenn ich sie mit der Hand schriebe, schienen sie von mir zu kommen, aber wenn sie mit der Maschine geschrieben wären, hätte er immer das Gefühl, dass sich die Maschine zwischen uns gestellt hätte.
- checker: adds unstated «mit der Hand» to disambiguate «when I wrote them», not present in source

**41. `h4-258`** · ch4 ¶60.0 · `minor` · status `disputed`

- EN: “Most certainly it does.
- DE: „Ganz gewiss tut es das.
- checker: «Ganz gewiss tut es das» is an unnatural calque of English do-support, not idiomatic German

**42. `h4-261`** · ch4 ¶67.6 · `minor` · status `disputed`

- EN: She pulled a little handkerchief out of her muff and began to sob heavily into it.
- DE: Sie zog ein kleines Taschentuch aus ihrem Muff und begann, heftig hineinzuweinen.
- checker: «hineinzuweinen» is an unusual/rare coinage for 'sob into it', not a standard German verb

**43. `h4-271`** · ch4 ¶87.8 · `minor` · status `disputed`

- EN: The sewing-machine, of the hand type, leaves a similar mark, but only on the left arm, and on the side of it farthest from the thumb, instead of being right across the broadest part, as this was.
- DE: Die Nähmaschine, von der Hand-Bauart, hinterlässt ein ähnliches Mal, aber nur am linken Arm und auf der dem Daumen am weitesten entfernten Seite, anstatt geradewegs über die breiteste Stelle zu verlaufen, wie es hier der Fall war.
- checker: «von der Hand-Bauart» is an awkward, non-standard compound for a hand-operated sewing machine

**44. `h4-275`** · ch4 ¶92.6 · `minor` · status `disputed`

- EN: Anybody bringing,” &c, &c.
- DE: Wer auch immer bringt ...‘ und so weiter, und so weiter.“
- checker: stray unmatched closing quote «...‘» has no corresponding opening mark

**45. `h4-287`** · ch4 ¶117.5 · `minor` · status `disputed`

- EN: In each case, not only are the ‘e’s’ slurred and the ‘r’s’ tailless, but you will observe, if you care to use my magnifying lens, that the fourteen other characteristics to which I have alluded are there as well.”
- DE: In jedem Fall sind nicht nur die ›e‹ verwischt und die ›r‹ schwanzlos, sondern Sie werden auch bemerken, wenn Sie meine Lupe benutzen möchten, dass die vierzehn anderen Merkmale, auf die ich angespielt habe, ebenfalls vorhanden sind.«
- checker: «schwanzlos» is an odd, non-standard coinage for a letter's missing tail

**46. `h4-291`** · ch4 ¶125.1 · `minor` · status `disputed`

- EN: It was a considerable sum, for people in their position, and the loss of it would have made a serious difference.
- DE: Es war eine beträchtliche Summe für Leute in ihrer Lage, und ihr Verlust hätte einen gravierenden Unterschied gemacht.
- checker: «einen gravierenden Unterschied gemacht» is an English calque, more natural German would use «hätte sich schwerwiegend ausgewirkt»

### Checker verdict `ok` (status `silver`)

**47. `h4-001`** · ch1 ¶0.0 · `ok` · status `silver`

- EN: The Adventures of Sherlock Holmes
- DE: Die Abenteuer des Sherlock Holmes

**48. `h4-003`** · ch2 ¶1.6 · `ok` · status `silver`

- EN: He never spoke of the softer passions, save with a gibe and a sneer.
- DE: Er sprach nie von den zarteren Leidenschaften, außer mit Spott und Hohn.

**49. `h4-004`** · ch2 ¶2.0 · `ok` · status `silver`

- EN: I had seen little of Holmes lately.
- DE: Ich hatte Holmes in letzter Zeit wenig gesehen.

**50. `h4-005`** · ch2 ¶2.5 · `ok` · status `silver`

- EN: Beyond these signs of his activity, however, which I merely shared with all the readers of the daily press, I knew little of my former friend and companion.
- DE: Außer diesen Zeichen seiner Tätigkeit jedoch, die ich bloß mit allen Lesern der Tagespresse teilte, wusste ich wenig von meinem einstigen Freund und Gefährten.

**51. `h4-006`** · ch2 ¶3.4 · `ok` · status `silver`

- EN: To me, who knew his every mood and habit, his attitude and manner told their own story.
- DE: Mir, der ich jede seiner Launen und Gewohnheiten kannte, sprachen seine Haltung und sein Verhalten eine deutliche Sprache.

**52. `h4-007`** · ch2 ¶4.1 · `ok` · status `silver`

- EN: It seldom was; but he was glad, I think, to see me.
- DE: Das war es selten; doch er freute sich, glaube ich, mich zu sehen.

**53. `h4-008`** · ch2 ¶5.1 · `ok` · status `silver`

- EN: “I think, Watson, that you have put on seven and a half pounds since I saw you.”
- DE: „Ich glaube, Watson, dass Sie siebeneinhalb Pfund zugenommen haben, seit ich Sie zuletzt sah.“

**54. `h4-010`** · ch2 ¶10.2 · `ok` · status `silver`

- EN: It is true that I had a country walk on Thursday and came home in a dreadful mess, but as I have changed my clothes I can’t imagine how you deduce it.
- DE: Es ist wahr, dass ich am Donnerstag einen Spaziergang über Land gemacht habe und in einem schrecklichen Zustand nach Hause kam, aber da ich mich umgezogen habe, kann ich mir nicht vorstellen, wie du es deduzierst.

**55. `h4-011`** · ch2 ¶12.2 · `ok` · status `silver`

- EN: Hence, you see, my double deduction that you had been out in vile weather, and that you had a particularly malignant boot-slitting specimen of the London slavey.
- DE: Daher rührt, siehst du, meine doppelte Deduktion, dass du bei scheußlichem Wetter draußen warst und dass du ein besonders bösartiges, stiefelritzendes Exemplar einer Londoner Magd hast.

**56. `h4-012`** · ch2 ¶14.0 · `ok` · status `silver`

- EN: “Quite so,” he answered, lighting a cigarette, and throwing himself down into an armchair.
- DE: „Ganz recht“, antwortete er, zündete sich eine Zigarette an und warf sich in einen Sessel.

**57. `h4-013`** · ch2 ¶18.0 · `ok` · status `silver`

- EN: “Then how many are there?”
- DE: „Wie viele sind es dann?“

**58. `h4-014`** · ch2 ¶20.4 · `ok` · status `silver`

- EN: Now, I know that there are seventeen steps, because I have both seen and observed.
- DE: Nun, ich weiß, dass es siebzehn Stufen sind, weil ich sowohl gesehen als auch beobachtet habe.

**59. `h4-015`** · ch2 ¶22.0 · `ok` · status `silver`

- EN: “There will call upon you to-night, at a quarter to eight o’clock,” it said, “a gentleman who desires to consult you upon a matter of the very deepest moment.
- DE: „Heute Abend, um Viertel vor acht Uhr“, hieß es darin, „wird Sie ein Herr aufsuchen, der Sie in einer Angelegenheit von allergrößter Bedeutung zu konsultieren wünscht.

**60. `h4-016`** · ch2 ¶23.1 · `ok` · status `silver`

- EN: “What do you imagine that it means?”
- DE: „Was glauben Sie, was das bedeutet?“

**61. `h4-017`** · ch2 ¶24.4 · `ok` · status `silver`

- EN: What do you deduce from it?”
- DE: Was deduzieren Sie daraus?“

**62. `h4-018`** · ch2 ¶27.0 · `ok` · status `silver`

- EN: “Peculiar—that is the very word,” said Holmes.
- DE: „Sonderbar – das ist genau das richtige Wort“, sagte Holmes.

**63. `h4-019`** · ch2 ¶30.0 · `ok` · status `silver`

- EN: “The name of the maker, no doubt; or his monogram, rather.”
- DE: „Der Name des Herstellers, zweifellos; oder vielmehr sein Monogramm.“

**64. `h4-020`** · ch2 ¶31.4 · `ok` · status `silver`

- EN: Now for the ‘Eg.’
- DE: Nun zu dem ‚Eg‘.

**65. `h4-021`** · ch2 ¶31.9 · `ok` · status `silver`

- EN: ‘Remarkable as being the scene of the death of Wallenstein, and for its numerous glass-factories and paper-mills.’
- DE: ‚Bemerkenswert als Schauplatz des Todes von Wallenstein und für seine zahlreichen Glasfabriken und Papiermühlen.‘

**66. `h4-024`** · ch2 ¶35.4 · `ok` · status `silver`

- EN: There’s money in this case, Watson, if there is nothing else.”
- DE: In diesem Fall steckt Geld, Watson, wenn auch sonst nichts.“

**67. `h4-025`** · ch2 ¶37.3 · `ok` · status `silver`

- EN: And this promises to be interesting.
- DE: Und dies verspricht interessant zu werden.

**68. `h4-026`** · ch2 ¶40.0 · `ok` · status `silver`

- EN: A slow and heavy step, which had been heard upon the stairs and in the passage, paused immediately outside the door.
- DE: Ein langsamer und schwerer Schritt, der auf der Treppe und im Korridor zu hören gewesen war, hielt unmittelbar vor der Tür inne.

**69. `h4-027`** · ch2 ¶42.2 · `ok` · status `silver`

- EN: Heavy bands of astrakhan were slashed across the sleeves and fronts of his double-breasted coat, while the deep blue cloak which was thrown over his shoulders was lined with flame-coloured silk and secured at the neck with a brooch which consisted of a single flaming beryl.
- DE: Schwere Bänder aus Astrachan waren quer über die Ärmel und die Vorderseite seines zweireihigen Mantels gezogen, während der tiefblaue Umhang, der über seine Schultern geworfen war, mit flammenfarbener Seide gefüttert und am Hals mit einer Brosche befestigt war, die aus einem einzigen flammenden Beryll bestand.

**70. `h4-028`** · ch2 ¶43.1 · `ok` · status `silver`

- EN: “I told you that I would call.”
- DE: „Ich hatte Ihnen ja mitgeteilt, dass ich vorsprechen würde.“

**71. `h4-030`** · ch2 ¶46.2 · `ok` · status `silver`

- EN: “You may say before this gentleman anything which you may say to me.”
- DE: „Sie können vor diesem Herrn alles sagen, was Sie mir sagen können.“

**72. `h4-031`** · ch2 ¶50.0 · `ok` · status `silver`

- EN: “You will excuse this mask,” continued our strange visitor.
- DE: „Sie werden diese Maske entschuldigen“, fuhr unser seltsamer Besucher fort.

**73. `h4-032`** · ch2 ¶52.1 · `ok` · status `silver`

- EN: To speak plainly, the matter implicates the great House of Ormstein, hereditary kings of Bohemia.”
- DE: Um offen zu sprechen: Die Angelegenheit betrifft das erhabene Haus Ormstein, die Erbkönige von Böhmen.“

**74. `h4-033`** · ch2 ¶56.0 · `ok` · status `silver`

- EN: The man sprang from his chair and paced up and down the room in uncontrollable agitation.
- DE: Der Mann sprang von seinem Stuhl auf und schritt in unkontrollierbarer Erregung im Zimmer auf und ab.

**75. `h4-035`** · ch2 ¶60.0 · `ok` · status `silver`

- EN: “The facts are briefly these: Some five years ago, during a lengthy visit to Warsaw, I made the acquaintance of the well-known adventuress, Irene Adler.
- DE: „Die Tatsachen sind kurz diese: Vor etwa fünf Jahren, während eines längeren Besuchs in Warschau, machte ich die Bekanntschaft der wohlbekannten Abenteurerin Irene Adler.

**76. `h4-037`** · ch2 ¶62.8 · `ok` · status `silver`

- EN: Your Majesty, as I understand, became entangled with this young person, wrote her some compromising letters, and is now desirous of getting those letters back.”
- DE: Eure Majestät hat sich, wie ich verstehe, mit dieser jungen Person eingelassen, ihr einige kompromittierende Briefe geschrieben und wünscht nun, diese Briefe zurückzubekommen.“

**77. `h4-038`** · ch2 ¶68.1 · `ok` · status `silver`

- EN: If this young person should produce her letters for blackmailing or other purposes, how is she to prove their authenticity?”
- DE: Wenn diese junge Person ihre Briefe zu Erpressungs- oder anderen Zwecken vorlegen sollte, wie soll sie deren Echtheit beweisen?“

**78. `h4-039`** · ch2 ¶80.0 · `ok` · status `silver`

- EN: “You have compromised yourself seriously.”
- DE: „Sie haben sich ernsthaft kompromittiert.“

**79. `h4-040`** · ch2 ¶84.0 · `ok` · status `silver`

- EN: “Your Majesty must pay.
- DE: „Eure Majestät muss zahlen.

**80. `h4-041`** · ch2 ¶87.2 · `ok` · status `silver`

- EN: Once we diverted her luggage when she travelled.
- DE: Einmal leiteten wir ihr Gepäck um, als sie reiste.

**81. `h4-042`** · ch2 ¶91.0 · `ok` · status `silver`

- EN: “But a very serious one to me,” returned the King reproachfully.
- DE: „Aber ein sehr ernstes für mich“, erwiderte der König vorwurfsvoll.

**82. `h4-043`** · ch2 ¶97.1 · `ok` · status `silver`

- EN: You may know the strict principles of her family.
- DE: Sie kennen vielleicht die strengen Prinzipien ihrer Familie.

**83. `h4-044`** · ch2 ¶99.1 · `ok` · status `silver`

- EN: And she will do it.
- DE: Und sie wird es tun.

**84. `h4-045`** · ch2 ¶100.0 · `ok` · status `silver`

- EN: “You are sure that she has not sent it yet?”
- DE: „Sie sind sicher, dass sie sie noch nicht geschickt hat?“

**85. `h4-046`** · ch2 ¶104.2 · `ok` · status `silver`

- EN: Your Majesty will, of course, stay in London for the present?”
- DE: Eure Majestät wird vorläufig natürlich in London bleiben?“

**86. `h4-047`** · ch2 ¶109.0 · `ok` · status `silver`

- EN: “You have carte blanche.”
- DE: „Sie haben freie Hand.“

**87. `h4-048`** · ch2 ¶115.0 · `ok` · status `silver`

- EN: Holmes scribbled a receipt upon a sheet of his note-book and handed it to him.
- DE: Holmes kritzelte eine Quittung auf ein Blatt seines Notizbuchs und reichte sie ihm.

**88. `h4-049`** · ch2 ¶118.2 · `ok` · status `silver`

- EN: “Was the photograph a cabinet?”
- DE: „War die Fotografie ein Kabinettformat?“

**89. `h4-050`** · ch2 ¶122.0 · `ok` · status `silver`

- EN: At three o’clock precisely I was at Baker Street, but Holmes had not yet returned.
- DE: Um Punkt drei Uhr war ich in der Baker Street, doch Holmes war noch nicht zurückgekehrt.

**90. `h4-051`** · ch2 ¶122.5 · `ok` · status `silver`

- EN: So accustomed was I to his invariable success that the very possibility of his failing had ceased to enter into my head.
- DE: So sehr war ich an seinen unveränderlichen Erfolg gewöhnt, dass mir die bloße Möglichkeit seines Scheiterns gar nicht mehr in den Sinn kam.

**91. `h4-052`** · ch2 ¶124.0 · `ok` · status `silver`

- EN: “Well, really!” he cried, and then he choked and laughed again until he was obliged to lie back, limp and helpless, in the chair.
- DE: „Also wirklich!“, rief er, und dann verschluckte er sich und lachte wieder, bis er gezwungen war, sich schlaff und hilflos im Sessel zurückzulehnen.

**92. `h4-053`** · ch2 ¶128.1 · `ok` · status `silver`

- EN: I will tell you, however.
- DE: Ich will es dir jedoch erzählen.

**93. `h4-054`** · ch2 ¶128.6 · `ok` · status `silver`

- EN: It is a bijou villa, with a garden at the back, but built out in front right up to the road, two stories.
- DE: Es ist eine Bijou-Villa mit einem Garten auf der Rückseite, aber nach vorne hin direkt bis an die Straße gebaut, zweistöckig.

**94. `h4-056`** · ch2 ¶131.1 · `ok` · status `silver`

- EN: She is the daintiest thing under a bonnet on this planet.
- DE: Sie ist das zierlichste Wesen unter einem Schutenhut auf diesem Planeten.

**95. `h4-057`** · ch2 ¶131.6 · `ok` · status `silver`

- EN: He is dark, handsome, and dashing, never calls less than once a day, and often twice.
- DE: Er ist dunkelhaarig, gutaussehend und schneidig, schaut nie seltener als einmal am Tag vorbei und oft zweimal.

**96. `h4-058`** · ch2 ¶132.0 · `ok` · status `silver`

- EN: “This Godfrey Norton was evidently an important factor in the matter.
- DE: Dieser Godfrey Norton war offensichtlich ein wichtiger Faktor in der Angelegenheit.

**97. `h4-061`** · ch2 ¶135.2 · `ok` · status `silver`

- EN: Presently he emerged, looking even more flurried than before.
- DE: Bald darauf kam er heraus, und sah noch aufgeregter aus als zuvor.

**98. `h4-063`** · ch2 ¶138.2 · `ok` · status `silver`

- EN: The driver looked twice at such a shabby fare, but I jumped in before he could object.
- DE: Der Fahrer sah sich einen so schäbigen Fahrgast zweimal an, aber ich sprang hinein, ehe er Einwände erheben konnte.

**99. `h4-064`** · ch2 ¶139.2 · `ok` · status `silver`

- EN: The cab and the landau with their steaming horses were in front of the door when I arrived.
- DE: Die Droschke und der Landauer mit ihren dampfenden Pferden standen vor der Tür, als ich ankam.

**100. `h4-065`** · ch2 ¶139.7 · `ok` · status `silver`

- EN: Suddenly, to my surprise, the three at the altar faced round to me, and Godfrey Norton came running as hard as he could towards me.
- DE: Plötzlich, zu meiner Überraschung, drehten sich die drei am Altar zu mir um, und Godfrey Norton kam so schnell er konnte auf mich zugerannt.

**101. `h4-066`** · ch2 ¶143.2 · `ok` · status `silver`

- EN: It was the most preposterous position in which I ever found myself in my life, and it was the thought of it that started me laughing just now.
- DE: Es war die absurdeste Lage, in der ich mich je in meinem Leben befunden habe, und es war der Gedanke daran, der mich eben zum Lachen brachte.

**102. `h4-067`** · ch2 ¶145.1 · `ok` · status `silver`

- EN: It looked as if the pair might take an immediate departure, and so necessitate very prompt and energetic measures on my part.
- DE: Es sah so aus, als ob das Paar unverzüglich abreisen könnte und somit sehr rasche und energische Maßnahmen meinerseits erforderlich machen würde.

**103. `h4-068`** · ch2 ¶145.5 · `ok` · status `silver`

- EN: They drove away in different directions, and I went off to make my own arrangements.”
- DE: Sie fuhren in verschiedene Richtungen davon, und ich machte mich auf den Weg, um meine eigenen Vorbereitungen zu treffen.“

**104. `h4-069`** · ch2 ¶149.0 · `ok` · status `silver`

- EN: “You don’t mind breaking the law?”
- DE: „Sie haben nichts dagegen, das Gesetz zu brechen?“

**105. `h4-070`** · ch2 ¶154.0 · `ok` · status `silver`

- EN: “Then I am your man.”
- DE: „Dann bin ich Ihr Mann.“

**106. `h4-071`** · ch2 ¶157.2 · `ok` · status `silver`

- EN: It is nearly five now.
- DE: Es ist jetzt fast fünf.

**107. `h4-073`** · ch2 ¶161.2 · `ok` · status `silver`

- EN: Do not join in it.
- DE: Beteiligen Sie sich nicht daran.

**108. `h4-074`** · ch2 ¶163.0 · `ok` · status `silver`

- EN: “You are to watch me, for I will be visible to you.”
- DE: „Sie sollen mich beobachten, denn ich werde für Sie sichtbar sein.“

**109. `h4-075`** · ch2 ¶167.2 · `ok` · status `silver`

- EN: Your task is confined to that.
- DE: Ihre Aufgabe beschränkt sich darauf.

**110. `h4-076`** · ch2 ¶170.0 · `ok` · status `silver`

- EN: “Then you may entirely rely on me.”
- DE: „Dann können Sie sich vollkommen auf mich verlassen.“

**111. `h4-078`** · ch2 ¶173.3 · `ok` · status `silver`

- EN: On the contrary, for a small street in a quiet neighbourhood, it was remarkably animated.
- DE: Im Gegenteil, für eine kleine Straße in einer ruhigen Nachbarschaft war sie bemerkenswert belebt.

**112. `h4-079`** · ch2 ¶174.3 · `ok` · status `silver`

- EN: Now the question is, Where are we to find the photograph?”
- DE: Nun ist die Frage: Wo sollen wir die Fotografie finden?“

**113. `h4-080`** · ch2 ¶176.3 · `ok` · status `silver`

- EN: She knows that the King is capable of having her waylaid and searched.
- DE: Sie weiß, dass der König fähig ist, ihr auflauern und sie durchsuchen zu lassen.

**114. `h4-081`** · ch2 ¶178.2 · `ok` · status `silver`

- EN: But I am inclined to think neither.
- DE: Aber ich neige dazu, an keines von beiden zu glauben.

**115. `h4-084`** · ch2 ¶186.2 · `ok` · status `silver`

- EN: It is her carriage.
- DE: Es ist ihre Kutsche.

**116. `h4-085`** · ch2 ¶187.3 · `ok` · status `silver`

- EN: A fierce quarrel broke out, which was increased by the two guardsmen, who took sides with one of the loungers, and by the scissors-grinder, who was equally hot upon the other side.
- DE: Ein heftiger Streit brach los, der durch die zwei Gardisten, die für einen der Müßiggänger Partei ergriffen, und durch den Scherenschleifer, der auf der anderen Seite ebenso hitzig war, noch verstärkt wurde.

**117. `h4-086`** · ch2 ¶187.7 · `ok` · status `silver`

- EN: Irene Adler, as I will still call her, had hurried up the steps; but she stood at the top with her superb figure outlined against the lights of the hall, looking back into the street.
- DE: Irene Adler, wie ich sie immer noch nennen will, war die Stufen hinaufgeeilt; doch sie stand oben, während sich ihre prachtvolle Gestalt gegen die Lichter der Halle abzeichnete, und blickte zurück auf die Straße.

**118. `h4-087`** · ch2 ¶191.0 · `ok` · status `silver`

- EN: “He’s a brave fellow,” said a woman.
- DE: „Er ist ein tapferer Kerl“, sagte eine Frau.

**119. `h4-088`** · ch2 ¶192.1 · `ok` · status `silver`

- EN: May we bring him in, marm?”
- DE: Dürfen wir ihn hineinbringen, Ma'am?“

**120. `h4-089`** · ch2 ¶194.2 · `ok` · status `silver`

- EN: I do not know whether he was seized with compunction at that moment for the part he was playing, but I know that I never felt more heartily ashamed of myself in my life than when I saw the beautiful creature against whom I was conspiring, or the grace and kindliness with which she waited upon the injured man.
- DE: Ich weiß nicht, ob ihn in jenem Moment Reue überkam wegen der Rolle, die er spielte, aber ich weiß, dass ich mich in meinem Leben nie von ganzem Herzen mehr geschämt habe als damals, als ich das wunderschöne Geschöpf sah, gegen das ich mich verschworen hatte, oder die Anmut und Herzlichkeit, mit der sie den Verletzten bediente.

**121. `h4-090`** · ch2 ¶195.1 · `ok` · status `silver`

- EN: A maid rushed across and threw open the window.
- DE: Ein Dienstmädchen eilte herbei und riss das Fenster auf.

**122. `h4-091`** · ch2 ¶195.6 · `ok` · status `silver`

- EN: Slipping through the shouting crowd I made my way to the corner of the street, and in ten minutes was rejoiced to find my friend’s arm in mine, and to get away from the scene of uproar.
- DE: Ich schlüpfte durch die schreiende Menge, bahnte mir meinen Weg zur Straßenecke und war nach zehn Minuten hocherfreut, den Arm meines Freundes in meinem zu spüren und den Schauplatz des Aufruhrs hinter mir zu lassen.

**123. `h4-092`** · ch2 ¶196.2 · `ok` · status `silver`

- EN: It is all right.”
- DE: Es ist alles in Ordnung.“

**124. `h4-093`** · ch2 ¶201.0 · `ok` · status `silver`

- EN: “I am still in the dark.”
- DE: „Ich tappe noch immer im Dunkeln.“

**125. `h4-094`** · ch2 ¶203.0 · `ok` · status `silver`

- EN: “I guessed as much.”
- DE: „Das habe ich mir schon gedacht.“

**126. `h4-095`** · ch2 ¶206.0 · `ok` · status `silver`

- EN: “Then they carried me in.
- DE: „Dann trugen sie mich hinein.

**127. `h4-097`** · ch2 ¶208.4 · `ok` · status `silver`

- EN: A married woman grabs at her baby; an unmarried one reaches for her jewel-box.
- DE: Eine verheiratete Frau greift nach ihrem Baby; eine unverheiratete greift nach ihrer Schmuckschatulle.

**128. `h4-098`** · ch2 ¶208.8 · `ok` · status `silver`

- EN: The smoke and shouting were enough to shake nerves of steel.
- DE: Der Rauch und das Geschrei genügten, um Nerven aus Stahl zu erschüttern.

**129. `h4-099`** · ch2 ¶208.14 · `ok` · status `silver`

- EN: I hesitated whether to attempt to secure the photograph at once; but the coachman had come in, and as he was watching me narrowly, it seemed safer to wait.
- DE: Ich zögerte, ob ich versuchen sollte, das Foto sofort an mich zu bringen; doch der Kutscher war hereingekommen, und da er mich scharf beobachtete, schien es sicherer zu sein, zu warten.

**130. `h4-100`** · ch2 ¶210.3 · `ok` · status `silver`

- EN: It might be a satisfaction to his Majesty to regain it with his own hands.”
- DE: Es könnte seiner Majestät eine Genugtuung sein, es mit eigenen Händen wiederzuerlangen.“

**131. `h4-101`** · ch2 ¶212.3 · `ok` · status `silver`

- EN: I must wire to the King without delay.”
- DE: Ich muss dem König ohne Verzögerung telegraphieren.“

**132. `h4-102`** · ch2 ¶216.0 · `ok` · status `silver`

- EN: “I’ve heard that voice before,” said Holmes, staring down the dimly lit street.
- DE: „Ich habe diese Stimme schon einmal gehört“, sagte Holmes und starrte die schwach beleuchtete Straße hinunter.

**133. `h4-103`** · ch2 ¶223.1 · `ok` · status `silver`

- EN: I am all impatience to be gone.”
- DE: Ich brenne vor Ungeduld aufzubrechen.“

**134. `h4-104`** · ch2 ¶226.1 · `ok` · status `silver`

- EN: We descended and started off once more for Briony Lodge.
- DE: Wir stiegen hinab und machten uns noch einmal auf den Weg zur Briony Lodge.

**135. `h4-105`** · ch2 ¶234.0 · `ok` · status `silver`

- EN: “And why in hopes?”
- DE: „Und warum diese Hoffnung?“

**136. `h4-106`** · ch2 ¶236.4 · `ok` · status `silver`

- EN: What a queen she would have made!”
- DE: Was für eine Königin sie abgegeben hätte!“

**137. `h4-107`** · ch2 ¶239.0 · `ok` · status `silver`

- EN: “I am Mr. Holmes,” answered my companion, looking at her with a questioning and rather startled gaze.
- DE: „Ich bin Herr Holmes“, antwortete mein Gefährte und sah sie mit einem fragenden und ziemlich überraschten Blick an.

**138. `h4-108`** · ch2 ¶243.0 · `ok` · status `silver`

- EN: “And the papers?” asked the King hoarsely.
- DE: „Und die Papiere?“, fragte der König heiser.

**139. `h4-109`** · ch2 ¶244.5 · `ok` · status `silver`

- EN: To be left till called for.”
- DE: Postlagernd.“

**140. `h4-110`** · ch2 ¶245.1 · `ok` · status `silver`

- EN: You took me in completely.
- DE: Sie haben mich völlig getäuscht.

**141. `h4-111`** · ch2 ¶245.6 · `ok` · status `silver`

- EN: And your address had been given me.
- DE: Und Ihre Adresse war mir gegeben worden.

**142. `h4-113`** · ch2 ¶245.16 · `ok` · status `silver`

- EN: I love and am loved by a better man than he.
- DE: Ich liebe einen besseren Mann als ihn und werde von ihm geliebt.

**143. `h4-114`** · ch2 ¶247.0 · `ok` · status `silver`

- EN: “What a woman—oh, what a woman!” cried the King of Bohemia, when we had all three read this epistle.
- DE: „Was für eine Frau – oh, was für eine Frau!“, rief der König von Böhmen, als wir alle drei diesen Brief gelesen hatten.

**144. `h4-115`** · ch2 ¶248.1 · `ok` · status `silver`

- EN: “I am sorry that I have not been able to bring your Majesty’s business to a more successful conclusion.”
- DE: „Es tut mir leid, dass ich nicht in der Lage gewesen bin, die Angelegenheit Eurer Majestät zu einem erfolgreicheren Abschluss zu bringen.“

**145. `h4-116`** · ch2 ¶250.0 · `ok` · status `silver`

- EN: “I am glad to hear your Majesty say so.”
- DE: „Es freut mich zu hören, dass Eure Majestät das sagt.“

**146. `h4-117`** · ch2 ¶253.0 · `ok` · status `silver`

- EN: “You have but to name it.”
- DE: „Sie brauchen es nur zu nennen.“

**147. `h4-118`** · ch2 ¶257.1 · `ok` · status `silver`

- EN: Then there is no more to be done in the matter.
- DE: Dann gibt es in der Angelegenheit nichts weiter zu tun.

**148. `h4-119`** · ch2 ¶258.2 · `ok` · status `silver`

- EN: And when he speaks of Irene Adler, or when he refers to her photograph, it is always under the honourable title of the woman.
- DE: Und wenn er von Irene Adler spricht oder wenn er sich auf ihre Fotografie bezieht, geschieht es stets unter dem ehrenvollen Titel der Frau.

**149. `h4-120`** · ch3 ¶5.0 · `ok` · status `silver`

- EN: “Then I can wait in the next room.”
- DE: „Dann kann ich im Nebenzimmer warten.“

**150. `h4-121`** · ch3 ¶8.2 · `ok` · status `silver`

- EN: You have shown your relish for it by the enthusiasm which has prompted you to chronicle, and, if you will excuse my saying so, somewhat to embellish so many of my own little adventures.”
- DE: Sie haben Ihren Gefallen daran durch die Begeisterung gezeigt, die Sie dazu veranlasst hat, so viele meiner eigenen kleinen Abenteuer aufzuzeichnen und, wenn Sie mir die Bemerkung verzeihen, ein wenig auszuschmücken.“

**151. `h4-122`** · ch3 ¶12.0 · `ok` · status `silver`

- EN: “You did, Doctor, but none the less you must come round to my view, for otherwise I shall keep on piling fact upon fact on you until your reason breaks down under them and acknowledges me to be right.
- DE: „Das taten Sie, Doktor, aber nichtsdestoweniger müssen Sie sich meiner Ansicht anschließen, denn andernfalls werde ich fortfahren, Tatsache auf Tatsache auf Sie zu häufen, bis Ihre Vernunft darunter zusammenbricht und mir Recht gibt.

**152. `h4-125`** · ch3 ¶15.0 · `ok` · status `silver`

- EN: Sherlock Holmes’ quick eye took in my occupation, and he shook his head with a smile as he noticed my questioning glances.
- DE: Sherlock Holmes’ rascher Blick erfasste meine Beschäftigung, und er schüttelte lächelnd den Kopf, als er meine fragenden Blicke bemerkte.

**153. `h4-126`** · ch3 ¶17.2 · `ok` · status `silver`

- EN: It’s as true as gospel, for I began as a ship’s carpenter.”
- DE: Es ist so wahr wie das Evangelium, denn ich begann als Schiffszimmermann.“

**154. `h4-127`** · ch3 ¶20.0 · `ok` · status `silver`

- EN: “I won’t insult your intelligence by telling you how I read that, especially as, rather against the strict rules of your order, you use an arc-and-compass breastpin.”
- DE: „Ich will Ihren Verstand nicht beleidigen, indem ich Ihnen erzähle, wie ich das abgelesen habe, zumal Sie, ziemlich entgegen den strengen Regeln Ihres Ordens, eine Zirkel-und-Bogen-Brustnadel tragen.“

**155. `h4-128`** · ch3 ¶24.1 · `ok` · status `silver`

- EN: I have made a small study of tattoo marks and have even contributed to the literature of the subject.
- DE: Ich habe eine kleine Studie über Tätowierungen betrieben und sogar zur Fachliteratur des Themas beigetragen.

**156. `h4-129`** · ch3 ¶25.2 · `ok` · status `silver`

- EN: “I thought at first that you had done something clever, but I see that there was nothing in it after all.”
- DE: „Ich dachte zuerst, Sie hätten etwas Kluges getan, aber ich sehe, dass schließlich gar nichts dahintersteckte.“

**157. `h4-130`** · ch3 ¶27.2 · `ok` · status `silver`

- EN: This is what began it all.
- DE: Das ist es, womit alles begann.

**158. `h4-131`** · ch3 ¶29.2 · `ok` · status `silver`

- EN: All red-headed men who are sound in body and mind and above the age of twenty-one years, are eligible.
- DE: Alle rothaarigen Männer, die an Körper und Geist gesund und über einundzwanzig Jahre alt sind, kommen in Betracht.

**159. `h4-132`** · ch3 ¶31.1 · `ok` · status `silver`

- EN: “It is a little off the beaten track, isn’t it?” said he.
- DE: „Es liegt ein wenig abseits der ausgetretenen Pfade, nicht wahr?“, sagte er.

**160. `h4-133`** · ch3 ¶34.0 · `ok` · status `silver`

- EN: “Well, it is just as I have been telling you, Mr. Sherlock Holmes,” said Jabez Wilson, mopping his forehead; “I have a small pawnbroker’s business at Coburg Square, near the City.
- DE: „Nun, es ist genau so, wie ich Ihnen erzählt habe, Herr Sherlock Holmes“, sagte Jabez Wilson und wischte sich die Stirn ab; „ich habe ein kleines Pfandleihergeschäft am Coburg Square, nahe der City.

**161. `h4-134`** · ch3 ¶36.0 · `ok` · status `silver`

- EN: “His name is Vincent Spaulding, and he’s not such a youth, either.
- DE: „Sein Name ist Vincent Spaulding, und so ein junger Mann ist er eigentlich auch wieder nicht.

**162. `h4-135`** · ch3 ¶37.2 · `ok` · status `silver`

- EN: It is not a common experience among employers in this age.
- DE: Es ist keine alltägliche Erfahrung unter Arbeitgebern in diesem Zeitalter.

**163. `h4-136`** · ch3 ¶38.3 · `ok` · status `silver`

- EN: That is his main fault, but on the whole he’s a good worker.
- DE: Das ist sein Hauptfehler, aber im Großen und Ganzen ist er ein guter Arbeiter.

**164. `h4-137`** · ch3 ¶41.0 · `ok` · status `silver`

- EN: “The first thing that put us out was that advertisement.
- DE: „Das Erste, was uns aus der Fassung brachte, war diese Anzeige.

**165. `h4-139`** · ch3 ¶46.1 · `ok` · status `silver`

- EN: ’ he asked with his eyes open.
- DE: fragte er mit großen Augen.

**166. `h4-141`** · ch3 ¶53.3 · `ok` · status `silver`

- EN: From all I hear it is splendid pay and very little to do.’
- DE: Nach allem, was ich höre, ist es eine glänzende Bezahlung und sehr wenig zu tun.‹

**167. `h4-142`** · ch3 ¶55.3 · `ok` · status `silver`

- EN: Then, again, I have heard it is no use your applying if your hair is light red, or dark red, or anything but real bright, blazing, fiery red.
- DE: Und dann habe ich auch gehört, dass es zwecklos ist, sich zu bewerben, wenn Ihr Haar hellrot oder dunkelrot ist oder irgendetwas anderes als echtes leuchtendes, flammendes, feuriges Rot.

**168. `h4-143`** · ch3 ¶57.0 · `ok` · status `silver`

- EN: “I never hope to see such a sight as that again, Mr. Holmes.
- DE: »Ich hoffe, einen solchen Anblick nie wieder sehen zu müssen, Herr Holmes.

**169. `h4-144`** · ch3 ¶57.5 · `ok` · status `silver`

- EN: When I saw how many were waiting, I would have given it up in despair; but Spaulding would not hear of it.
- DE: Als ich sah, wie viele warteten, hätte ich es verzweifelt aufgegeben; aber Spaulding wollte nichts davon hören.

**170. `h4-146`** · ch3 ¶60.0 · `ok` · status `silver`

- EN: “‘This is Mr. Jabez Wilson,’ said my assistant, ‘and he is willing to fill a vacancy in the League.’
- DE: „‚Das ist Herr Jabez Wilson‘, sagte mein Assistent, ‚und er ist bereit, eine freie Stelle im Bund zu besetzen.‘

**171. `h4-147`** · ch3 ¶61.4 · `ok` · status `silver`

- EN: Then suddenly he plunged forward, wrung my hand, and congratulated me warmly on my success.
- DE: Dann stürzte er plötzlich vorwärts, drückte mir die Hand und gratulierte mir herzlich zu meinem Erfolg.

**172. `h4-148`** · ch3 ¶62.4 · `ok` · status `silver`

- EN: ‘I perceive that all is as it should be.
- DE: ‚Ich sehe, dass alles so ist, wie es sein sollte.

**173. `h4-149`** · ch3 ¶63.0 · `ok` · status `silver`

- EN: “‘My name,’ said he, ‘is Mr. Duncan Ross, and I am myself one of the pensioners upon the fund left by our noble benefactor.
- DE: „‚Mein Name‘, sagte er, ‚ist Mr. Duncan Ross, und ich bin selbst einer der Pensionäre aus dem Fonds, den unser edler Wohltäter hinterlassen hat.

**174. `h4-150`** · ch3 ¶66.1 · `ok` · status `silver`

- EN: ’ he said gravely, ‘that is very serious indeed!
- DE: ‘, sagte er ernst, ‚das ist in der Tat sehr schwerwiegend!

**175. `h4-151`** · ch3 ¶68.0 · `ok` · status `silver`

- EN: “‘In the case of another,’ said he, ‘the objection might be fatal, but we must stretch a point in favour of a man with such a head of hair as yours.
- DE: „‚Im Falle eines anderen‘, sagte er, ‚könnte der Einwand verhängnisvoll sein, aber wir müssen bei einem Mann mit einer solchen Haarpracht wie der Ihren ein Auge zudrücken.

**176. `h4-152`** · ch3 ¶70.1 · `ok` · status `silver`

- EN: ’ said Vincent Spaulding.
- DE: ‘, sagte Vincent Spaulding.

**177. `h4-153`** · ch3 ¶74.0 · `ok` · status `silver`

- EN: “‘That would suit me very well,’ said I. ‘And the pay?’
- DE: „‚Das würde mir sehr gut passen‘, sagte ich. ‚Und die Bezahlung?‘

**178. `h4-154`** · ch3 ¶79.2 · `ok` · status `silver`

- EN: The will is very clear upon that point.
- DE: Das Testament ist in diesem Punkt sehr eindeutig.

**179. `h4-155`** · ch3 ¶83.0 · `ok` · status `silver`

- EN: “‘Is to copy out the Encyclopædia Britannica.
- DE: „‚Besteht darin, die Encyclopædia Britannica abzuschreiben.

**180. `h4-156`** · ch3 ¶85.1 · `ok` · status `silver`

- EN: ’ He bowed me out of the room and I went home with my assistant, hardly knowing what to say or do, I was so pleased at my own good fortune.
- DE: Er verbeugte sich und geleitete mich aus dem Zimmer, und ich ging mit meinem Gehilfen nach Hause, wobei ich kaum wusste, was ich sagen oder tun sollte, so sehr freute ich mich über mein eigenes großes Glück.

**181. `h4-157`** · ch3 ¶87.0 · `ok` · status `silver`

- EN: “Well, to my surprise and delight, everything was as right as possible.
- DE: „Nun, zu meiner Überraschung und Freude war alles in bester Ordnung.

**182. `h4-158`** · ch3 ¶88.0 · `ok` · status `silver`

- EN: “This went on day after day, Mr. Holmes, and on Saturday the manager came in and planked down four golden sovereigns for my week’s work.
- DE: „So ging es Tag für Tag weiter, Herr Holmes, und am Samstag kam der Geschäftsführer herein und blätterte vier goldene Sovereigns für meine Wochenarbeit hin.

**183. `h4-159`** · ch3 ¶89.0 · `ok` · status `silver`

- EN: “Eight weeks passed away like this, and I had written about Abbots and Archery and Armour and Architecture and Attica, and hoped with diligence that I might get on to the B’s before very long.
- DE: „Acht Wochen vergingen auf diese Weise, und ich hatte über Äbte und Bogenschießen und Rüstungen und Architektur und Attika geschrieben und hoffte bei entsprechendem Fleiß, bald zu den B's übergehen zu können.

**184. `h4-160`** · ch3 ¶91.3 · `ok` · status `silver`

- EN: Here it is, and you can read for yourself.”
- DE: Hier ist es, und Sie können es selbst lesen.“

**185. `h4-161`** · ch3 ¶95.0 · `ok` · status `silver`

- EN: “I cannot see that there is anything very funny,” cried our client, flushing up to the roots of his flaming head.
- DE: „Ich kann nicht erkennen, was daran so furchtbar komisch sein soll“, rief unser Klient, der bis zu den Wurzeln seines flammenden Haars errötete.

**186. `h4-163`** · ch3 ¶97.3 · `ok` · status `silver`

- EN: Finally, I went to the landlord, who is an accountant living on the ground floor, and I asked him if he could tell me what had become of the Red-headed League.
- DE: Schließlich ging ich zum Vermieter, der ein im Erdgeschoss wohnender Buchhalter ist, und ich fragte ihn, ob er mir sagen könne, was aus dem Bund der Rotköpfigen geworden sei.

**187. `h4-164`** · ch3 ¶98.0 · `ok` · status `silver`

- EN: “‘Well,’ said I, ‘the gentleman at No. 4.’
- DE: „‚Nun‘, sagte ich, ‚der Herr aus Nummer 4.‘

**188. `h4-165`** · ch3 ¶102.0 · `ok` · status `silver`

- EN: “‘Where could I find him?
- DE: „‚Wo könnte ich ihn finden?

**189. `h4-166`** · ch3 ¶105.0 · `ok` · status `silver`

- EN: “And what did you do then?” asked Holmes.
- DE: „Und was taten Sie dann?“, fragte Holmes.

**190. `h4-167`** · ch3 ¶106.4 · `ok` · status `silver`

- EN: I did not wish to lose such a place without a struggle, so, as I had heard that you were good enough to give advice to poor folk who were in need of it, I came right away to you.”
- DE: Ich wollte eine solche Stelle nicht kampflos aufgeben; und da ich gehört hatte, dass Sie so gütig seien, armen Leuten, die dessen bedürfen, Rat zu erteilen, kam ich geradewegs zu Ihnen.“

**191. `h4-168`** · ch3 ¶108.1 · `ok` · status `silver`

- EN: “Why, I have lost four pound a week.”
- DE: „Nun, ich habe vier Pfund die Woche verloren.“

**192. `h4-169`** · ch3 ¶111.0 · `ok` · status `silver`

- EN: “We shall endeavour to clear up these points for you.
- DE: „Wir werden uns bemühen, diese Punkte für Sie aufzuklären.

**193. `h4-170`** · ch3 ¶113.0 · `ok` · status `silver`

- EN: “How did he come?”
- DE: „Wie kam er zu Ihnen?“

**194. `h4-171`** · ch3 ¶118.0 · `ok` · status `silver`

- EN: “Because he was handy and would come cheap.”
- DE: „Weil er geschickt war und für wenig Geld arbeiten wollte.“

**195. `h4-172`** · ch3 ¶123.0 · `ok` · status `silver`

- EN: Holmes sat up in his chair in considerable excitement.
- DE: Holmes richtete sich in seinem Sessel in beträchtlicher Erregung auf.

**196. `h4-173`** · ch3 ¶125.1 · `ok` · status `silver`

- EN: “He is still with you?”
- DE: „Er ist immer noch bei Ihnen?“

**197. `h4-174`** · ch3 ¶129.0 · `ok` · status `silver`

- EN: “That will do, Mr. Wilson.
- DE: „Das genügt, Herr Wilson.

**198. `h4-175`** · ch3 ¶131.1 · `ok` · status `silver`

- EN: “It is a most mysterious business.”
- DE: „Es ist eine höchst rätselhafte Angelegenheit.“

**199. `h4-177`** · ch3 ¶135.0 · `ok` · status `silver`

- EN: “Sarasate plays at the St. James’s Hall this afternoon,” he remarked.
- DE: „Sarasate spielt heute Nachmittag in der St. James's Hall“, bemerkte er.

**200. `h4-178`** · ch3 ¶137.0 · `ok` · status `silver`

- EN: “Then put on your hat and come.
- DE: „Dann setzen Sie Ihren Hut auf und kommen Sie mit.

**201. `h4-180`** · ch3 ¶138.6 · `ok` · status `silver`

- EN: It was instantly opened by a bright-looking, clean-shaven young fellow, who asked him to step in.
- DE: Sie wurde sogleich von einem aufgeweckt aussehenden, glattrasierten jungen Burschen geöffnet, der ihn bat hereinzutreten.

**202. `h4-182`** · ch3 ¶146.0 · `ok` · status `silver`

- EN: “And what did you see?”
- DE: „Und was haben Sie gesehen?“

**203. `h4-184`** · ch3 ¶150.3 · `ok` · status `silver`

- EN: It was difficult to realise as we looked at the line of fine shops and stately business premises that they really abutted on the other side upon the faded and stagnant square which we had just quitted.
- DE: Es war schwer zu begreifen, als wir die Reihe feiner Läden und stattlicher Geschäftsräume betrachteten, dass sie auf der anderen Seite wirklich an den verblassten und stillen Platz grenzten, den wir gerade verlassen hatten.

**204. `h4-185`** · ch3 ¶151.4 · `ok` · status `silver`

- EN: And now, Doctor, we’ve done our work, so it’s time we had some play.
- DE: Und nun, Doktor, haben wir unsere Arbeit getan, also ist es Zeit, dass wir uns etwas Vergnügen gönnen.

**205. `h4-186`** · ch3 ¶152.3 · `ok` · status `silver`

- EN: The swing of his nature took him from extreme languor to devouring energy; and, as I knew well, he was never so truly formidable as when, for days on end, he had been lounging in his armchair amid his improvisations and his black-letter editions.
- DE: Das Pendeln seines Wesens führte ihn von äußerster Trägheit zu verzehrender Energie; und, wie ich wohl wusste, war er nie so wahrhaft furchterregend, wie wenn er tagelang inmitten seiner Improvisationen und seiner Frakturausgaben in seinem Lehnsessel gefaulenzt hatte.

**206. `h4-187`** · ch3 ¶155.0 · `ok` · status `silver`

- EN: “And I have some business to do which will take some hours.
- DE: „Und ich habe einige Besorgungen zu machen, die einige Stunden in Anspruch nehmen werden.

**207. `h4-188`** · ch3 ¶157.2 · `ok` · status `silver`

- EN: But to-day being Saturday rather complicates matters.
- DE: Aber dass heute Samstag ist, verkompliziert die Dinge ziemlich.

**208. `h4-189`** · ch3 ¶161.2 · `ok` · status `silver`

- EN: He waved his hand, turned on his heel, and disappeared in an instant among the crowd.
- DE: Er winkte mit der Hand, drehte sich auf dem Absatz um und verschwand im Nu in der Menge.

**209. `h4-190`** · ch3 ¶162.4 · `ok` · status `silver`

- EN: Where were we going, and what were we to do?
- DE: Wohin gingen wir, und was sollten wir tun?

**210. `h4-191`** · ch3 ¶163.2 · `ok` · status `silver`

- EN: On entering his room, I found Holmes in animated conversation with two men, one of whom I recognised as Peter Jones, the official police agent, while the other was a long, thin, sad-faced man, with a very shiny hat and oppressively respectable frock-coat.
- DE: Als ich sein Zimmer betrat, fand ich Holmes in lebhafter Unterhaltung mit zwei Männern, von denen ich einen als Peter Jones erkannte, den offiziellen Polizeibeamten, während der andere ein langer, dünner, traurig blickender Mann war, mit einem sehr glänzenden Hut und einem bedrückend respektablen Gehrock.

**211. `h4-193`** · ch3 ¶167.2 · `ok` · status `silver`

- EN: It is not too much to say that once or twice, as in that business of the Sholto murder and the Agra treasure, he has been more nearly correct than the official force.”
- DE: Es ist nicht zu viel gesagt, dass er ein- oder zweimal, wie in jener Angelegenheit des Sholto-Mordes und des Agra-Schatzes, der Wahrheit näher war als die offizielle Polizei.“

**212. `h4-194`** · ch3 ¶169.0 · `ok` · status `silver`

- EN: “I think you will find,” said Sherlock Holmes, “that you will play for a higher stake to-night than you have ever done yet, and that the play will be more exciting.
- DE: „Ich denke, Sie werden feststellen“, sagte Sherlock Holmes, „dass Sie heute Abend um einen höheren Einsatz spielen werden, als Sie es jemals getan haben, und dass das Spiel aufregender sein wird.

**213. `h4-195`** · ch3 ¶170.3 · `ok` · status `silver`

- EN: His grandfather was a royal duke, and he himself has been to Eton and Oxford.
- DE: Sein Großvater war ein königlicher Herzog, und er selbst ist in Eton und Oxford gewesen.

**214. `h4-196`** · ch3 ¶171.1 · `ok` · status `silver`

- EN: I’ve had one or two little turns also with Mr. John Clay, and I agree with you that he is at the head of his profession.
- DE: Ich habe auch schon ein oder zwei kleine Runden mit Herrn John Clay gehabt, und ich stimme mit Ihnen überein, dass er an der Spitze seines Fachs steht.

**215. `h4-197`** · ch3 ¶173.0 · `ok` · status `silver`

- EN: “We are close there now,” my friend remarked.
- DE: „Wir sind jetzt nah dran“, bemerkte mein Freund.

**216. `h4-198`** · ch3 ¶173.5 · `ok` · status `silver`

- EN: He is as brave as a bulldog and as tenacious as a lobster if he gets his claws upon anyone.
- DE: Er ist so mutig wie eine Bulldogge und so zäh wie ein Hummer, wenn er seine Scheren an jemanden legt.

**217. `h4-199`** · ch3 ¶174.3 · `ok` · status `silver`

- EN: This also was opened, and led down a flight of winding stone steps, which terminated at another formidable gate.
- DE: Auch dieses wurde geöffnet und führte eine gewundene Steintreppe hinab, die an einem weiteren gewaltigen Tor endete.

**218. `h4-200`** · ch3 ¶176.1 · `ok` · status `silver`

- EN: “Why, dear me, it sounds quite hollow!” he remarked, looking up in surprise.
- DE: „Ei, du meine Güte, es klingt ganz hohl!“, bemerkte er und blickte überrascht auf.

**219. `h4-201`** · ch3 ¶178.1 · `ok` · status `silver`

- EN: A few seconds sufficed to satisfy him, for he sprang to his feet again and put his glass in his pocket.
- DE: Einige Sekunden genügten, um ihn zufriedenzustellen, denn er sprang wieder auf die Beine und steckte sein Glas in die Tasche.

**220. `h4-204`** · ch3 ¶184.0 · `ok` · status `silver`

- EN: “And sit in the dark?”
- DE: „Und im Dunkeln sitzen?“

**221. `h4-206`** · ch3 ¶186.0 · `ok` · status `silver`

- EN: I placed my revolver, cocked, upon the top of the wooden case behind which I crouched.
- DE: Ich legte meinen Revolver, gespannt, oben auf die Holzkiste, hinter der ich kauerte.

**222. `h4-209`** · ch3 ¶191.0 · `ok` · status `silver`

- EN: At first it was but a lurid spark upon the stone pavement.
- DE: Zunächst war es nur ein unheimlicher Funke auf dem Steinpflaster.

**223. `h4-210`** · ch3 ¶192.1 · `ok` · status `silver`

- EN: With a rending, tearing sound, one of the broad, white stones turned over upon its side and left a square, gaping hole, through which streamed the light of a lantern.
- DE: Mit einem berstenden, reißenden Laut drehte sich einer der breiten, weißen Steine auf die Seite und hinterließ ein quadratisches, gähnendes Loch, durch welches das Licht einer Laterne strömte.

**224. `h4-211`** · ch3 ¶193.3 · `ok` · status `silver`

- EN: Jump, Archie, jump, and I’ll swing for it!”
- DE: Spring, Archie, spring, und ich hänge dafür!“

**225. `h4-213`** · ch3 ¶198.1 · `ok` · status `silver`

- EN: You seem to have done the thing very completely.
- DE: Sie scheinen die Sache sehr gründlich gemacht zu haben.

**226. `h4-214`** · ch3 ¶200.1 · `ok` · status `silver`

- EN: “He’s quicker at climbing down holes than I am.
- DE: „Er klettert schneller in Löcher hinab als ich.

**227. `h4-216`** · ch3 ¶204.1 · `ok` · status `silver`

- EN: There is no doubt that you have detected and defeated in the most complete manner one of the most determined attempts at bank robbery that have ever come within my experience.”
- DE: Es besteht kein Zweifel, dass Sie auf die vollkommenste Weise einen der entschlossensten Versuche eines Bankraubs aufgedeckt und vereitelt haben, der mir je in meiner Erfahrung begegnet ist.“

**228. `h4-217`** · ch3 ¶206.2 · `ok` · status `silver`

- EN: The method was no doubt suggested to Clay’s ingenious mind by the colour of his accomplice’s hair.
- DE: Die Methode wurde Clays erfinderischem Verstand zweifellos durch die Farbe des Haares seines Komplizen eingegeben.

**229. `h4-218`** · ch3 ¶207.0 · `ok` · status `silver`

- EN: “But how could you guess what the motive was?”
- DE: „Aber wie konnten Sie erraten, was das Motiv war?“

**230. `h4-219`** · ch3 ¶208.4 · `ok` · status `silver`

- EN: What could it be?
- DE: Was konnte es sein?

**231. `h4-220`** · ch3 ¶208.10 · `ok` · status `silver`

- EN: What could it be, once more?
- DE: Was konnte es abermals sein?

**232. `h4-221`** · ch3 ¶209.3 · `ok` · status `silver`

- EN: It was not in front.
- DE: Er lag nicht vorn.

**233. `h4-222`** · ch3 ¶209.8 · `ok` · status `silver`

- EN: You must yourself have remarked how worn, wrinkled, and stained they were.
- DE: Sie müssen selbst bemerkt haben, wie abgewetzt, verknittert und fleckig sie waren.

**234. `h4-223`** · ch3 ¶210.0 · `ok` · status `silver`

- EN: “And how could you tell that they would make their attempt to-night?”
- DE: „Und woher konnten Sie wissen, dass sie ihren Versuch heute Nacht unternehmen würden?“

**235. `h4-224`** · ch3 ¶211.3 · `ok` · status `silver`

- EN: For all these reasons I expected them to come to-night.”
- DE: Aus all diesen Gründen erwartete ich, dass sie heute Nacht kommen würden.“

**236. `h4-225`** · ch3 ¶213.3 · `ok` · status `silver`

- EN: My life is spent in one long effort to escape from the commonplaces of existence.
- DE: Mein Leben vergeht in einem einzigen langen Bemühen, den Alltäglichkeiten des Daseins zu entfliehen.

**237. `h4-226`** · ch3 ¶215.2 · `ok` · status `silver`

- EN: “‘L’homme c’est rien—l’œuvre c’est tout,’ as Gustave Flaubert wrote to George Sand.”
- DE: „‚L’homme c’est rien — l’œuvre c’est tout‘, wie Gustave Flaubert an George Sand schrieb.“

**238. `h4-227`** · ch4 ¶2.0 · `ok` · status `silver`

- EN: “And yet I am not convinced of it,” I answered.
- DE: „Und doch bin ich davon nicht überzeugt“, antwortete ich.

**239. `h4-228`** · ch4 ¶3.2 · `ok` · status `silver`

- EN: Depend upon it, there is nothing so unnatural as the commonplace.”
- DE: Verlassen Sie sich darauf, es gibt nichts so Unnatürliches wie das Alltägliche.“

**240. `h4-229`** · ch4 ¶4.4 · `ok` · status `silver`

- EN: Here is the first heading upon which I come.
- DE: Hier ist die erste Überschrift, auf die ich stoße.

**241. `h4-231`** · ch4 ¶6.0 · `ok` · status `silver`

- EN: He held out his snuffbox of old gold, with a great amethyst in the centre of the lid.
- DE: Er hielt seine Schnupftabakdose aus Altgold hin, mit einem großen Amethysten in der Mitte des Deckels.

**242. `h4-233`** · ch4 ¶11.2 · `ok` · status `silver`

- EN: Indeed, I have found that it is usually in unimportant matters that there is a field for the observation, and for the quick analysis of cause and effect which gives the charm to an investigation.
- DE: In der Tat habe ich festgestellt, dass es gewöhnlich bei unwichtigen Angelegenheiten ein Feld für die Beobachtung gibt und für die rasche Analyse von Ursache und Wirkung, die einer Untersuchung den Reiz verleiht.

**243. `h4-235`** · ch4 ¶13.2 · `ok` · status `silver`

- EN: She would like advice, but is not sure that the matter is not too delicate for communication.
- DE: Sie möchte gern einen Rat, ist sich aber nicht sicher, ob die Angelegenheit nicht zu heikel ist, um mitgeteilt zu werden.

**244. `h4-236`** · ch4 ¶13.6 · `ok` · status `silver`

- EN: But here she comes in person to resolve our doubts.”
- DE: Aber da kommt sie schon selbst, um unsere Zweifel zu zerstreuen.“

**245. `h4-237`** · ch4 ¶16.1 · `ok` · status `silver`

- EN: Then, suddenly realising the full purport of his words, she gave a violent start and looked up, with fear and astonishment upon her broad, good-humoured face.
- DE: Dann, als sie plötzlich die volle Tragweite seiner Worte begriff, fuhr sie heftig zusammen und blickte auf, mit Furcht und Erstaunen auf ihrem breiten, gutmütigen Gesicht.

**246. `h4-238`** · ch4 ¶18.0 · `ok` · status `silver`

- EN: “I came to you, sir, because I heard of you from Mrs. Etherege, whose husband you found so easy when the police and everyone had given him up for dead.
- DE: „Ich bin zu Ihnen gekommen, Sir, weil ich von Ihnen durch Mrs. Etherege gehört habe, deren Mann Sie so leicht fanden, als die Polizei und jedermann ihn schon für tot gehalten hatten.

**247. `h4-240`** · ch4 ¶24.0 · `ok` · status `silver`

- EN: “Oh, yes, mother is alive and well.
- DE: „Oh ja, Mutter lebt und ist wohlauf.

**248. `h4-241`** · ch4 ¶26.0 · `ok` · status `silver`

- EN: “Your own little income,” he asked, “does it come out of the business?”
- DE: „Ihr eigenes kleines Einkommen“, fragte er, „stammt das aus dem Geschäft?“

**249. `h4-242`** · ch4 ¶28.0 · `ok` · status `silver`

- EN: “You interest me extremely,” said Holmes.
- DE: „Sie interessieren mich ungemein“, sagte Holmes.

**250. `h4-243`** · ch4 ¶29.2 · `ok` · status `silver`

- EN: Mr. Windibank draws my interest every quarter and pays it over to mother, and I find that I can do pretty well with what I earn at typewriting.
- DE: Herr Windibank hebt meine Zinsen jedes Quartal ab und übergibt sie Mutter, und ich finde, dass ich mit dem, was ich durch Maschineschreiben verdiene, recht gut zurechtkomme.

**251. `h4-244`** · ch4 ¶31.0 · `ok` · status `silver`

- EN: A flush stole over Miss Sutherland’s face, and she picked nervously at the fringe of her jacket.
- DE: Eine Rötung stieg Miss Sutherland ins Gesicht, und sie zupfte nervös an den Fransen ihrer Jacke.

**252. `h4-245`** · ch4 ¶31.5 · `ok` · status `silver`

- EN: He would get quite mad if I wanted so much as to join a Sunday-school treat.
- DE: Er wurde ganz rasend, wenn ich auch nur an einem Sonntagsschulausflug teilnehmen wollte.

**253. `h4-246`** · ch4 ¶32.0 · `ok` · status `silver`

- EN: “I suppose,” said Holmes, “that when Mr. Windibank came back from France he was very annoyed at your having gone to the ball.”
- DE: „Ich nehme an“, sagte Holmes, „dass Herr Windibank, als er aus Frankreich zurückkehrte, sehr verärgert darüber war, dass Sie auf den Ball gegangen waren.“

**254. `h4-247`** · ch4 ¶37.0 · `ok` · status `silver`

- EN: “Well, you know father didn’t like anything of the sort.
- DE: „Nun, Sie wissen ja, Vater mochte so etwas überhaupt nicht.

**255. `h4-248`** · ch4 ¶38.1 · `ok` · status `silver`

- EN: Did he make no attempt to see you?”
- DE: Machte er keinen Versuch, Sie zu sehen?“

**256. `h4-249`** · ch4 ¶41.0 · `ok` · status `silver`

- EN: “Oh, yes, Mr. Holmes.
- DE: „Oh, ja, Herr Holmes.

**257. `h4-250`** · ch4 ¶45.0 · `ok` · status `silver`

- EN: “He slept on the premises.”
- DE: „Er schlief in den Geschäftsräumen.“

**258. `h4-252`** · ch4 ¶51.0 · `ok` · status `silver`

- EN: “He was a very shy man, Mr. Holmes.
- DE: „Er war ein sehr schüchterner Mann, Herr Holmes.

**259. `h4-253`** · ch4 ¶51.5 · `ok` · status `silver`

- EN: He was always well dressed, very neat and plain, but his eyes were weak, just as mine are, and he wore tinted glasses against the glare.”
- DE: Er war immer gut gekleidet, sehr ordentlich und schlicht, aber seine Augen waren schwach, genau wie die meinen, und er trug getönte Gläser gegen das grelle Licht.“

**260. `h4-254`** · ch4 ¶53.2 · `ok` · status `silver`

- EN: Mother said he was quite right to make me swear, and that it was a sign of his passion.
- DE: Mutter sagte, er habe völlig recht, mich schwören zu lassen, und dass es ein Zeichen seiner Leidenschaft sei.

**261. `h4-255`** · ch4 ¶54.0 · `ok` · status `silver`

- EN: “It missed him, then?”
- DE: „Er verfehlte ihn also?“

**262. `h4-256`** · ch4 ¶57.0 · `ok` · status `silver`

- EN: “Yes, sir, but very quietly.
- DE: „Ja, mein Herr, aber sehr still.

**263. `h4-257`** · ch4 ¶57.5 · `ok` · status `silver`

- EN: That was last Friday, Mr. Holmes, and I have never seen or heard anything since then to throw any light upon what became of him.”
- DE: Das war letzten Freitag, Herr Holmes, und ich habe seither weder etwas gesehen noch gehört, was irgendein Licht darauf werfen könnte, was aus ihm geworden ist.“

**264. `h4-259`** · ch4 ¶64.1 · `ok` · status `silver`

- EN: How did your mother take the matter?”
- DE: Wie hat Ihre Mutter die Angelegenheit aufgenommen?“

**265. `h4-260`** · ch4 ¶67.1 · `ok` · status `silver`

- EN: As he said, what interest could anyone have in bringing me to the doors of the church, and then leaving me?
- DE: Wie er sagte: Welches Interesse könnte jemand daran haben, mich bis an die Kirchentüren zu bringen und mich dann zu verlassen?

**266. `h4-262`** · ch4 ¶71.0 · `ok` · status `silver`

- EN: “Then what has happened to him?”
- DE: „Was ist ihm denn dann zugestoßen?“

**267. `h4-263`** · ch4 ¶75.0 · `ok` · status `silver`

- EN: “No. 31 Lyon Place, Camberwell.”
- DE: „Lyon Place Nr. 31, Camberwell.“

**268. `h4-264`** · ch4 ¶78.2 · `ok` · status `silver`

- EN: You will leave the papers here, and remember the advice which I have given you.
- DE: Sie lassen die Papiere hier und denken an den Rat, den ich Ihnen gegeben habe.

**269. `h4-265`** · ch4 ¶80.0 · `ok` · status `silver`

- EN: For all the preposterous hat and the vacuous face, there was something noble in the simple faith of our visitor which compelled our respect.
- DE: Trotz des lächerlichen Hutes und des ausdruckslosen Gesichts lag etwas Edles in dem einfachen Glauben unserer Besucherin, das unseren Respekt erzwang.

**270. `h4-266`** · ch4 ¶82.0 · `ok` · status `silver`

- EN: “Quite an interesting study, that maiden,” he observed.
- DE: „Eine recht interessante Studie, dieses Mädchen“, bemerkte er.

**271. `h4-267`** · ch4 ¶83.0 · `ok` · status `silver`

- EN: “You appeared to read a good deal upon her which was quite invisible to me,” I remarked.
- DE: „Sie schienen eine ganze Menge an ihr abzulesen, was für mich völlig unsichtbar war“, bemerkte ich.

**272. `h4-268`** · ch4 ¶85.0 · `ok` · status `silver`

- EN: “Well, she had a slate-coloured, broad-brimmed straw hat, with a feather of a brickish red.
- DE: „Nun, sie trug einen schieferfarbenen, breitkrempigen Strohhut mit einer ziegelroten Feder.

**273. `h4-269`** · ch4 ¶85.5 · `ok` · status `silver`

- EN: She had small round, hanging gold earrings, and a general air of being fairly well-to-do in a vulgar, comfortable, easy-going way.”
- DE: Sie trug kleine runde, herabhängende goldene Ohrringe und erweckte den allgemeinen Eindruck, auf eine vulgäre, bequeme, unbeschwerte Art ziemlich wohlhabend zu sein.“

**274. `h4-270`** · ch4 ¶87.3 · `ok` · status `silver`

- EN: Never trust to general impressions, my boy, but concentrate yourself upon details.
- DE: Verlassen Sie sich niemals auf allgemeine Eindrücke, mein Junge, sondern konzentrieren Sie sich auf Einzelheiten.

**275. `h4-272`** · ch4 ¶89.2 · `ok` · status `silver`

- EN: One was buttoned only in the two lower buttons out of five, and the other at the first, third, and fifth.
- DE: Der eine war nur an den zwei unteren von fünf Knöpfen zugeknöpft und der andere am ersten, dritten und fünften.

**276. `h4-273`** · ch4 ¶91.2 · `ok` · status `silver`

- EN: She had written in a hurry and dipped her pen too deep.
- DE: Sie hatte in Eile geschrieben und ihre Feder zu tief eingetaucht.

**277. `h4-274`** · ch4 ¶92.1 · `ok` · status `silver`

- EN: “Missing,” it said, “on the morning of the fourteenth, a gentleman named Hosmer Angel.
- DE: „Vermisst“, hieß es dort, „am Morgen des Vierzehnten, ein Herr namens Hosmer Angel.

**278. `h4-276`** · ch4 ¶94.0 · `ok` · status `silver`

- EN: “They are typewritten,” I remarked.
- DE: „Sie sind mit der Schreibmaschine geschrieben“, bemerkte ich.

**279. `h4-277`** · ch4 ¶97.0 · `ok` · status `silver`

- EN: “My dear fellow, is it possible you do not see how strongly it bears upon the case?”
- DE: „Mein lieber Freund, ist es möglich, dass Sie nicht sehen, wie stark sich das auf den Fall auswirkt?“

**280. `h4-278`** · ch4 ¶99.2 · `ok` · status `silver`

- EN: One is to a firm in the City, the other is to the young lady’s stepfather, Mr. Windibank, asking him whether he could meet us here at six o’clock to-morrow evening.
- DE: Der eine geht an eine Firma in der City, der andere an den Stiefvater der jungen Dame, Herrn Windibank, mit der Bitte, uns morgen Abend um sechs Uhr hier zu treffen.

**281. `h4-279`** · ch4 ¶101.0 · `ok` · status `silver`

- EN: I left him then, still puffing at his black clay pipe, with the conviction that when I came again on the next evening I would find that he held in his hands all the clues which would lead up to the identity of the disappearing bridegroom of Miss Mary Sutherland.
- DE: Ich verließ ihn daraufhin, immer noch an seiner schwarzen Tonpfeife paffend, mit der Überzeugung, dass ich, wenn ich am nächsten Abend wiederkäme, feststellen würde, dass er alle Fäden in Händen hielte, die zur Identität des verschwundenen Bräutigams von Fräulein Mary Sutherland führen würden.

**282. `h4-280`** · ch4 ¶103.0 · `ok` · status `silver`

- EN: “Well, have you solved it?”
- DE: „Nun, haben Sie es gelöst?“

**283. `h4-281`** · ch4 ¶106.2 · `ok` · status `silver`

- EN: There was never any mystery in the matter, though, as I said yesterday, some of the details are of interest.
- DE: Es gab in der Angelegenheit nie ein Geheimnis, obwohl, wie ich gestern sagte, einige der Einzelheiten von Interesse sind.

**284. `h4-282`** · ch4 ¶109.1 · `ok` · status `silver`

- EN: “He has written to me to say that he would be here at six.
- DE: „Er hat mir geschrieben, um mitzuteilen, dass er um sechs Uhr hier sein würde.

**285. `h4-283`** · ch4 ¶112.1 · `ok` · status `silver`

- EN: I am afraid that I am a little late, but I am not quite my own master, you know.
- DE: Ich fürchte, ich bin ein wenig spät dran, aber ich bin nicht ganz mein eigener Herr, wie Sie wissen.

**286. `h4-284`** · ch4 ¶112.5 · `ok` · status `silver`

- EN: Besides, it is a useless expense, for how could you possibly find this Hosmer Angel?”
- DE: Außerdem ist es eine nutzlose Ausgabe, denn wie könnten Sie diesen Hosmer Angel möglicherweise finden?“

**287. `h4-285`** · ch4 ¶115.1 · `ok` · status `silver`

- EN: Unless they are quite new, no two of them write exactly alike.
- DE: Wenn sie nicht ganz neu sind, schreiben keine zwei von ihnen genau gleich.

**288. `h4-286`** · ch4 ¶117.0 · `ok` · status `silver`

- EN: “And now I will show you what is really a very interesting study, Mr. Windibank,” Holmes continued.
- DE: „Und nun will ich Ihnen zeigen, was wirklich eine sehr interessante Studie ist, Herr Windibank“, fuhr Holmes fort.

**289. `h4-288`** · ch4 ¶119.1 · `ok` · status `silver`

- EN: “I let you know, then, that I have caught him!”
- DE: »Ich lasse Sie hiermit wissen, dass ich ihn gefasst habe!«

**290. `h4-289`** · ch4 ¶121.4 · `ok` · status `silver`

- EN: Sit down and let us talk it over.”
- DE: Setzen Sie sich und lassen Sie es uns besprechen.«

**291. `h4-290`** · ch4 ¶123.1 · `ok` · status `silver`

- EN: But between ourselves, Windibank, it was as cruel and selfish and heartless a trick in a petty way as ever came before me.
- DE: Aber unter uns gesagt, Windibank, es war ein so grausamer, selbstsüchtiger und herzloser Streich im Kleinen, wie er mir je untergekommen ist.

**292. `h4-292`** · ch4 ¶125.6 · `ok` · status `silver`

- EN: But soon he found that that would not answer forever.
- DE: Doch bald stellte er fest, dass dies nicht ewig genügen würde.

**293. `h4-293`** · ch4 ¶126.0 · `ok` · status `silver`

- EN: “It was only a joke at first,” groaned our visitor.
- DE: „Es war anfangs nur ein Scherz“, stöhnte unser Besucher.

**294. `h4-294`** · ch4 ¶127.4 · `ok` · status `silver`

- EN: There were meetings, and an engagement, which would finally secure the girl’s affections from turning towards anyone else.
- DE: Es gab Treffen und eine Verlobung, die die Zuneigung des Mädchens endgültig davor bewahren sollte, sich jemand anderem zuzuwenden.

**295. `h4-295`** · ch4 ¶127.9 · `ok` · status `silver`

- EN: James Windibank wished Miss Sutherland to be so bound to Hosmer Angel, and so uncertain as to his fate, that for ten years to come, at any rate, she would not listen to another man.
- DE: James Windibank wünschte Miss Sutherland so an Hosmer Angel gebunden und so im Ungewissen über sein Schicksal zu sehen, dass sie für die nächsten zehn Jahre jedenfalls keinem anderen Mann Gehör schenken würde.

**296. `h4-296`** · ch4 ¶129.0 · `ok` · status `silver`

- EN: “It may be so, or it may not, Mr. Holmes,” said he, “but if you are so very sharp you ought to be sharp enough to know that it is you who are breaking the law now, and not me. I have done nothing actionable from the first, but as long as you keep that door locked you lay yourself open to an action for assault and illegal constraint.”
- DE: „Es mag so sein oder auch nicht, Mr. Holmes“, sagte er, „aber wenn Sie so überaus schlau sind, sollten Sie schlau genug sein zu wissen, dass Sie es sind, der jetzt das Gesetz bricht, und nicht ich. Ich habe von Anfang an nichts Klagbares getan, aber solange Sie diese Tür verschlossen halten, setzen Sie sich einer Klage wegen tätlichen Angriffs und rechtswidriger Freiheitsberaubung aus.“

**297. `h4-297`** · ch4 ¶131.0 · `ok` · status `silver`

- EN: “There’s a cold-blooded scoundrel!” said Holmes, laughing, as he threw himself down into his chair once more.
- DE: „Das ist ein kaltblütiger Schurke!“, sagte Holmes lachend, während er sich wieder in seinen Sessel warf.

**298. `h4-298`** · ch4 ¶133.1 · `ok` · status `silver`

- EN: Then the fact that the two men were never together, but that the one always appeared when the other was away, was suggestive.
- DE: Dann war die Tatsache, dass die beiden Männer nie zusammen waren, sondern der eine immer dann erschien, wenn der andere fort war, vielsagend.

**299. `h4-299`** · ch4 ¶135.0 · `ok` · status `silver`

- EN: “Having once spotted my man, it was easy to get corroboration.
- DE: „Nachdem ich meinen Mann erst einmal ausgemacht hatte, war es leicht, eine Bestätigung zu erhalten.

**300. `h4-300`** · ch4 ¶135.5 · `ok` · status `silver`

- EN: As I expected, his reply was typewritten and revealed the same trivial but characteristic defects.
- DE: Wie ich erwartet hatte, war seine Antwort mit der Schreibmaschine geschrieben und offenbarte dieselben geringfügigen, aber charakteristischen Mängel.

