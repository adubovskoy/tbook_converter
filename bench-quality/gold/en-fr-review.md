# GOLD-lite review — en→fr (French)

Set: `bench-quality/gold/en-fr.jsonl` — 300 sentences, *The Adventures of Sherlock Holmes*, chapters 1–4.

Reference translations: **google/gemini-3.7-flash**, no repair pass. Adversarial check: **claude sonnet (via the claude CLI)**, one verdict per sentence.

Checker verdicts: **wrong 3**, **minor 38**, **ok 259**.

## How to use this file

**No one on the team reads French.** This file exists for provenance and for a
future reader; nothing in it is promoted to `gold` by this pass. Every item keeps
the machine verdict: `silver` where the adversarial checker said `ok`,
`disputed` where it said `minor` or `wrong`. Treat the whole set as **silver**:
usable as an absolute regression alarm, never as a ground truth.

## Items

### Disputed — checker verdict `wrong`

**1. `h4-062`** · ch2 ¶136.1 · `wrong` · status `disputed`

- EN: It hadn’t pulled up before she shot out of the hall door and into it.
- FR: Elle ne s’était pas encore arrêtée qu’elle jaillit de la porte du vestibule et monta dedans.
- checker: «Elle ne s'était pas encore arrêtée» wrongly makes the woman the subject of stopping, when the source «It hadn't pulled up» refers to the cab

**2. `h4-087`** · ch2 ¶191.0 · `wrong` · status `disputed`

- EN: “He’s a brave fellow,” said a woman.
- FR: « C'est un brave homme », dit une femme.
- checker: «un brave homme» (adjective before noun) means a good/kind man, not a courageous one as in «brave fellow»

**3. `h4-240`** · ch4 ¶24.0 · `wrong` · status `disputed`

- EN: “Oh, yes, mother is alive and well.
- FR: « Oh, oui, mère est vivante et bien portante.
- checker: «mère est vivante» drops the required possessive: standard French needs «ma mère», not a bare noun as subject

### Disputed — checker verdict `minor`

**4. `h4-003`** · ch2 ¶1.6 · `minor` · status `disputed`

- EN: He never spoke of the softer passions, save with a gibe and a sneer.
- FR: Il ne parlait jamais des passions les plus douces, sauf avec un sarcasme et un ricanement.
- checker: «les plus douces» is a superlative (softest), shifting the comparative sense of «softer passions»

**5. `h4-037`** · ch2 ¶62.8 · `minor` · status `disputed`

- EN: Your Majesty, as I understand, became entangled with this young person, wrote her some compromising letters, and is now desirous of getting those letters back.”
- FR: Votre Majesté, si j'ai bien compris, s'est compromise avec cette jeune personne, lui a écrit quelques lettres compromettantes, et est maintenant désireuse de récupérer ces lettres. »
- checker: «s'est compromise» sharpens «became entangled with» into an explicit admission of scandal not yet stated

**6. `h4-052`** · ch2 ¶124.0 · `minor` · status `disputed`

- EN: “Well, really!” he cried, and then he choked and laughed again until he was obliged to lie back, limp and helpless, in the chair.
- FR: « Eh bien, vraiment ! » s’écria-t-il, puis il s’étouffa et rit de nouveau jusqu’à ce qu’il fût contraint de se renverser en arrière, mou et impuissant, dans son fauteuil.
- checker: «impuissant» (usually powerless/impotent) is an odd collocation for limp-with-laughter «helpless»

**7. `h4-053`** · ch2 ¶128.1 · `minor` · status `disputed`

- EN: I will tell you, however.
- FR: Je vais vous raconter, toutefois.
- checker: «Je vais vous raconter» lacks a direct object (raconter quoi?), leaving the sentence incomplete

**8. `h4-054`** · ch2 ¶128.6 · `minor` · status `disputed`

- EN: It is a bijou villa, with a garden at the back, but built out in front right up to the road, two stories.
- FR: C’est une villa bijou, avec un jardin à l’arrière, mais bâtie à l’avant jusqu’au bord même de la route, à deux étages.
- checker: «à deux étages» overstates height: French étage-counting excludes ground floor, so English «two stories» maps to «un étage», not two

**9. `h4-066`** · ch2 ¶143.2 · `minor` · status `disputed`

- EN: It was the most preposterous position in which I ever found myself in my life, and it was the thought of it that started me laughing just now.
- FR: C’était la situation la plus absurde dans laquelle je me sois jamais trouvé de ma vie, et c’est le souvenir de cela qui m’a fait rire tout à l’heure.
- checker: «le souvenir de cela» (the memory of it) shifts the meaning of «the thought of it»

**10. `h4-067`** · ch2 ¶145.1 · `minor` · status `disputed`

- EN: It looked as if the pair might take an immediate departure, and so necessitate very prompt and energetic measures on my part.
- FR: On aurait dit que le couple pouvait prendre un départ immédiat, et ainsi nécessiter des mesures très promptes et énergiques de ma part.
- checker: «prendre un départ immédiat» is an awkward calque; French would say «partir immédiatement»

**11. `h4-080`** · ch2 ¶176.3 · `minor` · status `disputed`

- EN: She knows that the King is capable of having her waylaid and searched.
- FR: Elle sait que le roi est capable de la faire guetter et fouiller.
- checker: «guetter» means to watch/lurk, softer than «waylaid» (intercept/ambush her)

**12. `h4-082`** · ch2 ¶178.7 · `minor` · status `disputed`

- EN: It must be where she can lay her hands upon it.
- FR: Elle doit se trouver là où elle peut mettre la main dessus.
- checker: double use of «elle» for both the photo and Irene creates ambiguous reference

**13. `h4-083`** · ch2 ¶182.0 · `minor` · status `disputed`

- EN: “I will not look.”
- FR: « Je ne chercherai pas. »
- checker: «chercherai» (will search) shifts the sense from simply not looking/glancing

**14. `h4-092`** · ch2 ¶196.2 · `minor` · status `disputed`

- EN: It is all right.”
- FR: Tout est parfait. »
- checker: «Tout est parfait» (everything is perfect) overstates the plain reassurance of «It is all right»

**15. `h4-096`** · ch2 ¶206.5 · `minor` · status `disputed`

- EN: They laid me on a couch, I motioned for air, they were compelled to open the window, and you had your chance.”
- FR: Ils m'ont allongé sur un canapé, j'ai fait signe d'avoir besoin d'air, ils ont été contraints d'ouvrir la fenêtre, et vous avez eu votre chance. »
- checker: «j'ai fait signe d'avoir besoin d'air» is an awkward, non-idiomatic use of «faire signe de + infinitif»

**16. `h4-105`** · ch2 ¶234.0 · `minor` · status `disputed`

- EN: “And why in hopes?”
- FR: « Et pourquoi l'espoir ? »
- checker: «pourquoi l'espoir» nominalizes the idiom awkwardly, losing the force of 'in hopes of'

**17. `h4-121`** · ch3 ¶8.2 · `minor` · status `disputed`

- EN: You have shown your relish for it by the enthusiasm which has prompted you to chronicle, and, if you will excuse my saying so, somewhat to embellish so many of my own little adventures.”
- FR: Vous avez montré votre penchant pour cela par l'enthousiasme qui vous a poussé à faire la chronique et, si vous me permettez de le dire, à embellir quelque peu tant de mes propres petites aventures. »
- checker: «faire la chronique et ... embellir tant de mes propres petites aventures» drops the required preposition «de» after «faire la chronique»

**18. `h4-124`** · ch3 ¶14.0 · `minor` · status `disputed`

- EN: I did not gain very much, however, by my inspection.
- FR: Je ne gagnai pas grand-chose, cependant, à mon inspection.
- checker: «gagner à mon inspection» is an awkward collocation for 'gain by my inspection'

**19. `h4-127`** · ch3 ¶20.0 · `minor` · status `disputed`

- EN: “I won’t insult your intelligence by telling you how I read that, especially as, rather against the strict rules of your order, you use an arc-and-compass breastpin.”
- FR: « Je ne ferai pas l’injure à votre intelligence de vous dire comment j’ai lu cela, d’autant plus que, tout à fait à l’encontre des règles strictes de votre ordre, vous portez une épingle de cravate à l’équerre et au compas. »
- checker: «à l'équerre» (square) mistranslates «arc» (bow-shaped) from «arc-and-compass»

**20. `h4-140`** · ch3 ¶51.0 · `minor` · status `disputed`

- EN: “Well, you can easily think that that made me prick up my ears, for the business has not been over good for some years, and an extra couple of hundred would have been very handy.
- FR: « Eh bien, vous pensez bien que cela m'a fait dresser les oreilles, car les affaires n'allaient pas trop bien depuis quelques années, et deux cents livres de plus auraient été très commodes.
- checker: «livres» invents a currency unit not stated in the English «couple of hundred»

**21. `h4-146`** · ch3 ¶60.0 · `minor` · status `disputed`

- EN: “‘This is Mr. Jabez Wilson,’ said my assistant, ‘and he is willing to fill a vacancy in the League.’
- FR: « “Voici M. Jabez Wilson”, dit mon assistant, “et il désire pourvoir une vacance au sein de la Ligue.”
- checker: «pourvoir une vacance» is an awkward collocation; usual phrasing is «pourvoir un poste vacant» or «combler une vacance»

**22. `h4-156`** · ch3 ¶85.1 · `minor` · status `disputed`

- EN: ’ He bowed me out of the room and I went home with my assistant, hardly knowing what to say or do, I was so pleased at my own good fortune.
- FR: ” Il me reconduisit jusqu'à la porte avec une révérence et je rentrai chez moi avec mon assistant, sachant à peine que dire ou faire, tant j'étais heureux de ma propre bonne fortune.
- checker: «sachant à peine que dire ou faire» needs «quoi» not «que» without a preceding «ne»

**23. `h4-158`** · ch3 ¶88.0 · `minor` · status `disputed`

- EN: “This went on day after day, Mr. Holmes, and on Saturday the manager came in and planked down four golden sovereigns for my week’s work.
- FR: « Cela dura jour après jour, monsieur Holmes, et le samedi, le gérant entra et posa net quatre souverains d'or pour ma semaine de travail.
- checker: «posa net» is an unidiomatic collocation for money being slapped down

**24. `h4-160`** · ch3 ¶91.3 · `minor` · status `disputed`

- EN: Here it is, and you can read for yourself.”
- FR: Le voici, et vous pouvez lire par vous-même. »
- checker: «vous pouvez lire par vous-même» drops the object pronoun «le» referring to the will

**25. `h4-179`** · ch3 ¶138.1 · `minor` · status `disputed`

- EN: It was a poky, little, shabby-genteel place, where four lines of dingy two-storied brick houses looked out into a small railed-in enclosure, where a lawn of weedy grass and a few clumps of faded laurel bushes made a hard fight against a smoke-laden and uncongenial atmosphere.
- FR: C’était un endroit étriqué, petit, d’une médiocrité prétentieuse, où quatre rangées de maisons de briques décaties à deux étages donnaient sur un petit enclos grillagé, où une pelouse d’herbes folles et quelques touffes de lauriers fanés luttaient vaillamment contre une atmosphère enfumée et hostile.
- checker: «railed-in enclosure» (iron railings) rendered as «enclos grillagé» (wire mesh), a different physical image

**26. `h4-181`** · ch3 ¶141.2 · `minor` · status `disputed`

- EN: I have known something of him before.”
- FR: J'ai déjà eu affaire à lui par le passé. »
- checker: «eu affaire à lui» implies prior dealings, stronger than merely 'known something of him'

**27. `h4-183`** · ch3 ¶149.2 · `minor` · status `disputed`

- EN: We know something of Saxe-Coburg Square.
- FR: Nous savons quelque chose de Saxe-Cobourg Square.
- checker: «Saxe-Cobourg» respells the proper noun «Saxe-Coburg» from the source

**28. `h4-191`** · ch3 ¶163.2 · `minor` · status `disputed`

- EN: On entering his room, I found Holmes in animated conversation with two men, one of whom I recognised as Peter Jones, the official police agent, while the other was a long, thin, sad-faced man, with a very shiny hat and oppressively respectable frock-coat.
- FR: En entrant dans sa chambre, je trouvai Holmes en conversation animée avec deux hommes, dont l'un fut reconnu par moi comme étant Peter Jones, l'agent de police officiel, tandis que l'autre était un homme grand, mince, au visage triste, avec un chapeau très brillant et une redingote d'une respectabilité écrasante.
- checker: «fut reconnu par moi comme étant» is an unnatural English-style passive; French would say «que je reconnus comme étant»

**29. `h4-199`** · ch3 ¶174.3 · `minor` · status `disputed`

- EN: This also was opened, and led down a flight of winding stone steps, which terminated at another formidable gate.
- FR: Celle-ci fut également ouverte, et menait en bas d'un escalier de pierre en colimaçon, qui aboutissait à une autre grille formidable.
- checker: «menait en bas d'un escalier» shifts meaning from 'led down a flight of steps' to 'led to the bottom of a staircase'

**30. `h4-230`** · ch4 ¶4.8 · `minor` · status `disputed`

- EN: The crudest of writers could invent nothing more crude.”
- FR: Le plus fruste des écrivains ne pourrait rien inventer de plus banal. »
- checker: second «crude» rendered as «banal» (trite) instead of matching «fruste», losing the rhetorical repetition and shifting meaning

**31. `h4-234`** · ch4 ¶12.1 · `minor` · status `disputed`

- EN: Looking over his shoulder, I saw that on the pavement opposite there stood a large woman with a heavy fur boa round her neck, and a large curling red feather in a broad-brimmed hat which was tilted in a coquettish Duchess of Devonshire fashion over her ear.
- FR: Regardant par-dessus son épaule, je vis que sur le trottoir d'en face se tenait une femme corpulente avec un lourd boa de fourrure autour du cou, et une grande plume rouge recourbée dans un chapeau à larges bords qui était incliné d'une façon coquette à la duchesse de Devonshire sur son oreille.
- checker: «d'une façon coquette à la duchesse de Devonshire» awkwardly stacks two style markers, blurring the intended «à la Duchesse de Devonshire» idiom

**32. `h4-247`** · ch4 ¶37.0 · `minor` · status `disputed`

- EN: “Well, you know father didn’t like anything of the sort.
- FR: « Eh bien, vous savez que père n'aimait rien de ce genre.
- checker: «père» used bare without possessive/article sounds odd for standard French, though intentional archaic style is plausible

**33. `h4-260`** · ch4 ¶67.1 · `minor` · status `disputed`

- EN: As he said, what interest could anyone have in bringing me to the doors of the church, and then leaving me?
- FR: Comme il le disait, quel intérêt quiconque pourrait-il avoir à m’amener jusqu’aux portes de l’église, pour ensuite m’abandonner ?
- checker: «m'abandonner» adds an emotional charge not present in the neutral «leaving me»

**34. `h4-261`** · ch4 ¶67.6 · `minor` · status `disputed`

- EN: She pulled a little handkerchief out of her muff and began to sob heavily into it.
- FR: Elle tira un petit mouchoir de son manchon et commença à sangloter lourdement dedans.
- checker: «sangloter lourdement dedans» is an awkward calque, more natural French would drop or rephrase «dedans»

**35. `h4-265`** · ch4 ¶80.0 · `minor` · status `disputed`

- EN: For all the preposterous hat and the vacuous face, there was something noble in the simple faith of our visitor which compelled our respect.
- FR: Malgré tout le chapeau absurde et le visage vacant, il y avait dans la foi simple de notre visiteuse quelque chose de noble qui commandait notre respect.
- checker: «Malgré tout le chapeau absurde» is an awkward calque of 'for all the... hat', reads oddly in French

**36. `h4-269`** · ch4 ¶85.5 · `minor` · status `disputed`

- EN: She had small round, hanging gold earrings, and a general air of being fairly well-to-do in a vulgar, comfortable, easy-going way.”
- FR: Elle avait de petites boucles d'oreilles pendantes rondes en or, et un air général d'être assez aisée d'une manière vulgaire, confortable et décontractée. »
- checker: «un air général d'être assez aisée» is an awkward English-calqued construction rather than idiomatic French

**37. `h4-271`** · ch4 ¶87.8 · `minor` · status `disputed`

- EN: The sewing-machine, of the hand type, leaves a similar mark, but only on the left arm, and on the side of it farthest from the thumb, instead of being right across the broadest part, as this was.
- FR: La machine à coudre, du modèle à main, laisse une marque similaire, mais seulement sur le bras gauche, et du côté de celui-ci le plus éloigné du pouce, au lieu d'être en plein à travers la partie la plus large, comme c'était le cas ici.
- checker: «en plein à travers» is an awkward, non-idiomatic collocation for 'right across'

**38. `h4-277`** · ch4 ¶97.0 · `minor` · status `disputed`

- EN: “My dear fellow, is it possible you do not see how strongly it bears upon the case?”
- FR: « Mon cher ami, est-il possible que vous ne voyiez pas à quel point cela pèse sur l'affaire ? »
- checker: «pèse sur l'affaire» shifts 'bears upon' (is relevant to) toward 'weighs/burdens', a slight nuance drift

**39. `h4-290`** · ch4 ¶123.1 · `minor` · status `disputed`

- EN: But between ourselves, Windibank, it was as cruel and selfish and heartless a trick in a petty way as ever came before me.
- FR: Mais entre nous, Windibank, c'était un tour aussi cruel, égoïste et sans cœur dans sa mesquinerie que j'en aie jamais vu. »
- checker: «que j'en aie jamais vu» uses subjunctive after a plain equality comparison, missing the expected «ne» explétif, making the clause sound ungrammatical

**40. `h4-291`** · ch4 ¶125.1 · `minor` · status `disputed`

- EN: It was a considerable sum, for people in their position, and the loss of it would have made a serious difference.
- FR: C'était une somme considérable, pour des gens de leur condition, et sa perte aurait fait une sérieuse différence.
- checker: «une sérieuse différence» is an English calque; French would say «une grande/notable différence»

**41. `h4-297`** · ch4 ¶131.0 · `minor` · status `disputed`

- EN: “There’s a cold-blooded scoundrel!” said Holmes, laughing, as he threw himself down into his chair once more.
- FR: « En voilà un scélérat de sang-froid ! » dit Holmes en riant, tandis qu'il se laissait tomber à nouveau dans son fauteuil.
- checker: «scélérat de sang-froid» is an unusual collocation; «de sang-froid» normally modifies an action (kill in cold blood), not a person's character

### Checker verdict `ok` (status `silver`)

**42. `h4-001`** · ch1 ¶0.0 · `ok` · status `silver`

- EN: The Adventures of Sherlock Holmes
- FR: Les Aventures de Sherlock Holmes

**43. `h4-002`** · ch2 ¶1.1 · `ok` · status `silver`

- EN: I have seldom heard him mention her under any other name.
- FR: Je l'ai rarement entendu la mentionner sous un autre nom.

**44. `h4-004`** · ch2 ¶2.0 · `ok` · status `silver`

- EN: I had seen little of Holmes lately.
- FR: J’avais peu vu Holmes ces derniers temps.

**45. `h4-005`** · ch2 ¶2.5 · `ok` · status `silver`

- EN: Beyond these signs of his activity, however, which I merely shared with all the readers of the daily press, I knew little of my former friend and companion.
- FR: Au-delà de ces marques de son activité, toutefois, que je partageais simplement avec tous les lecteurs de la presse quotidienne, je ne savais que peu de choses de mon ancien ami et compagnon.

**46. `h4-006`** · ch2 ¶3.4 · `ok` · status `silver`

- EN: To me, who knew his every mood and habit, his attitude and manner told their own story.
- FR: Pour moi, qui connaissais chacune de ses humeurs et de ses habitudes, son attitude et ses manières racontaient leur propre histoire.

**47. `h4-007`** · ch2 ¶4.1 · `ok` · status `silver`

- EN: It seldom was; but he was glad, I think, to see me.
- FR: Elle l'était rarement ; mais il était content, je crois, de me voir.

**48. `h4-008`** · ch2 ¶5.1 · `ok` · status `silver`

- EN: “I think, Watson, that you have put on seven and a half pounds since I saw you.”
- FR: « Je pense, Watson, que vous avez pris sept livres et demie depuis que je vous ai vu. »

**49. `h4-009`** · ch2 ¶8.0 · `ok` · status `silver`

- EN: “Then, how do you know?”
- FR: « Alors, comment le savez-vous ? »

**50. `h4-010`** · ch2 ¶10.2 · `ok` · status `silver`

- EN: It is true that I had a country walk on Thursday and came home in a dreadful mess, but as I have changed my clothes I can’t imagine how you deduce it.
- FR: Il est vrai que j'ai fait une promenade à la campagne jeudi et que je suis rentré dans un état épouvantable, mais comme j'ai changé de vêtements, je ne peux pas imaginer comment vous le déduisez.

**51. `h4-011`** · ch2 ¶12.2 · `ok` · status `silver`

- EN: Hence, you see, my double deduction that you had been out in vile weather, and that you had a particularly malignant boot-slitting specimen of the London slavey.
- FR: De là, voyez-vous, ma double déduction que vous étiez sorti par un temps affreux, et que vous aviez un spécimen particulièrement malfaisant et lacérateur de bottes de la boniche de Londres.

**52. `h4-012`** · ch2 ¶14.0 · `ok` · status `silver`

- EN: “Quite so,” he answered, lighting a cigarette, and throwing himself down into an armchair.
- FR: « Tout à fait, répondit-il en allumant une cigarette et en se laissant tomber dans un fauteuil.

**53. `h4-013`** · ch2 ¶18.0 · `ok` · status `silver`

- EN: “Then how many are there?”
- FR: « Alors, combien y en a-t-il ? »

**54. `h4-014`** · ch2 ¶20.4 · `ok` · status `silver`

- EN: Now, I know that there are seventeen steps, because I have both seen and observed.
- FR: Or, je sais qu'il y a dix-sept marches, parce que j'ai à la fois vu et observé.

**55. `h4-015`** · ch2 ¶22.0 · `ok` · status `silver`

- EN: “There will call upon you to-night, at a quarter to eight o’clock,” it said, “a gentleman who desires to consult you upon a matter of the very deepest moment.
- FR: « Il se présentera chez vous ce soir, à huit heures moins le quart, disait-il, un monsieur qui désire vous consulter sur une affaire de la plus haute importance.

**56. `h4-016`** · ch2 ¶23.1 · `ok` · status `silver`

- EN: “What do you imagine that it means?”
- FR: « Que supposez-vous que cela signifie ? »

**57. `h4-017`** · ch2 ¶24.4 · `ok` · status `silver`

- EN: What do you deduce from it?”
- FR: Qu'en déduisez-vous ? »

**58. `h4-018`** · ch2 ¶27.0 · `ok` · status `silver`

- EN: “Peculiar—that is the very word,” said Holmes.
- FR: « Particulier... c'est le mot exact », dit Holmes.

**59. `h4-019`** · ch2 ¶30.0 · `ok` · status `silver`

- EN: “The name of the maker, no doubt; or his monogram, rather.”
- FR: « Le nom du fabricant, sans doute ; ou plutôt son monogramme. »

**60. `h4-020`** · ch2 ¶31.4 · `ok` · status `silver`

- EN: Now for the ‘Eg.’
- FR: Venons-en maintenant au « Eg ».

**61. `h4-021`** · ch2 ¶31.9 · `ok` · status `silver`

- EN: ‘Remarkable as being the scene of the death of Wallenstein, and for its numerous glass-factories and paper-mills.’
- FR: “Remarquable comme étant le lieu de la mort de Wallenstein, et pour ses nombreuses verreries et papeteries.”

**62. `h4-022`** · ch2 ¶33.2 · `ok` · status `silver`

- EN: Do you note the peculiar construction of the sentence—‘This account of you we have from all quarters received.
- FR: Remarquez-vous la construction particulière de la phrase — “Ce rapport sur vous nous avons de tous côtés reçu.

**63. `h4-023`** · ch2 ¶34.0 · `ok` · status `silver`

- EN: As he spoke there was the sharp sound of horses’ hoofs and grating wheels against the curb, followed by a sharp pull at the bell.
- FR: Tandis qu'il parlait, il y eut le son net de sabots de chevaux et de roues grinçant contre la bordure du trottoir, suivi d'un coup sec au cordon de sonnette.

**64. `h4-024`** · ch2 ¶35.4 · `ok` · status `silver`

- EN: There’s money in this case, Watson, if there is nothing else.”
- FR: Il y a de l’argent dans cette affaire, Watson, s’il n’y a rien d’autre. »

**65. `h4-025`** · ch2 ¶37.3 · `ok` · status `silver`

- EN: And this promises to be interesting.
- FR: Et cela promet d’être intéressant.

**66. `h4-026`** · ch2 ¶40.0 · `ok` · status `silver`

- EN: A slow and heavy step, which had been heard upon the stairs and in the passage, paused immediately outside the door.
- FR: Un pas lent et lourd, qu'on avait entendu dans l'escalier et dans le couloir, s'arrêta juste derrière la porte.

**67. `h4-027`** · ch2 ¶42.2 · `ok` · status `silver`

- EN: Heavy bands of astrakhan were slashed across the sleeves and fronts of his double-breasted coat, while the deep blue cloak which was thrown over his shoulders was lined with flame-coloured silk and secured at the neck with a brooch which consisted of a single flaming beryl.
- FR: De larges bandes d'astrakan barraient les manches et le devant de son manteau croisé, tandis que la cape d'un bleu profond jetée sur ses épaules était doublée de soie couleur de flamme et retenue au cou par une broche faite d'un unique béryl flamboyant.

**68. `h4-028`** · ch2 ¶43.1 · `ok` · status `silver`

- EN: “I told you that I would call.”
- FR: Je vous avais prévenu que je passerais. »

**69. `h4-029`** · ch2 ¶45.0 · `ok` · status `silver`

- EN: “You may address me as the Count Von Kramm, a Bohemian nobleman.
- FR: « Vous pouvez m'appeler le comte von Kramm, un noble de Bohême.

**70. `h4-030`** · ch2 ¶46.2 · `ok` · status `silver`

- EN: “You may say before this gentleman anything which you may say to me.”
- FR: Vous pouvez dire devant ce monsieur tout ce que vous pouvez me dire à moi. »

**71. `h4-031`** · ch2 ¶50.0 · `ok` · status `silver`

- EN: “You will excuse this mask,” continued our strange visitor.
- FR: « Vous excuserez ce masque, continua notre étrange visiteur.

**72. `h4-032`** · ch2 ¶52.1 · `ok` · status `silver`

- EN: To speak plainly, the matter implicates the great House of Ormstein, hereditary kings of Bohemia.”
- FR: Pour parler clairement, l'affaire implique la grande Maison d'Ormstein, rois héréditaires de Bohême. »

**73. `h4-033`** · ch2 ¶56.0 · `ok` · status `silver`

- EN: The man sprang from his chair and paced up and down the room in uncontrollable agitation.
- FR: L'homme bondit de sa chaise et fit les cent pas dans la pièce avec une agitation incontrôlable.

**74. `h4-034`** · ch2 ¶57.1 · `ok` · status `silver`

- EN: “Your Majesty had not spoken before I was aware that I was addressing Wilhelm Gottsreich Sigismond von Ormstein, Grand Duke of Cassel-Felstein, and hereditary King of Bohemia.”
- FR: « Votre Majesté n'avait pas encore parlé que je savais déjà que je m'adressais à Wilhelm Gottsreich Sigismond von Ormstein, grand-duc de Cassel-Felstein et roi héréditaire de Bohême. »

**75. `h4-035`** · ch2 ¶60.0 · `ok` · status `silver`

- EN: “The facts are briefly these: Some five years ago, during a lengthy visit to Warsaw, I made the acquaintance of the well-known adventuress, Irene Adler.
- FR: « Les faits sont brièvement les suivants : Il y a environ cinq ans, au cours d'un long séjour à Varsovie, j'ai fait la connaissance de la célèbre aventurière, Irene Adler.

**76. `h4-036`** · ch2 ¶62.0 · `ok` · status `silver`

- EN: “Let me see!” said Holmes.
- FR: « Voyons voir ! » dit Holmes.

**77. `h4-038`** · ch2 ¶68.1 · `ok` · status `silver`

- EN: If this young person should produce her letters for blackmailing or other purposes, how is she to prove their authenticity?”
- FR: Si cette jeune personne venait à produire ses lettres à des fins de chantage ou pour d'autres motifs, comment prouverait-elle leur authenticité ? »

**78. `h4-039`** · ch2 ¶80.0 · `ok` · status `silver`

- EN: “You have compromised yourself seriously.”
- FR: « Vous vous êtes gravement compromis. »

**79. `h4-040`** · ch2 ¶84.0 · `ok` · status `silver`

- EN: “Your Majesty must pay.
- FR: « Votre Majesté doit payer.

**80. `h4-041`** · ch2 ¶87.2 · `ok` · status `silver`

- EN: Once we diverted her luggage when she travelled.
- FR: Une fois nous avons détourné ses bagages lorsqu'elle voyageait.

**81. `h4-042`** · ch2 ¶91.0 · `ok` · status `silver`

- EN: “But a very serious one to me,” returned the King reproachfully.
- FR: « Mais un problème très sérieux pour moi », répliqua le roi d'un ton de reproche.

**82. `h4-043`** · ch2 ¶97.1 · `ok` · status `silver`

- EN: You may know the strict principles of her family.
- FR: Vous connaissez peut-être les principes stricts de sa famille.

**83. `h4-044`** · ch2 ¶99.1 · `ok` · status `silver`

- EN: And she will do it.
- FR: Et elle le fera.

**84. `h4-045`** · ch2 ¶100.0 · `ok` · status `silver`

- EN: “You are sure that she has not sent it yet?”
- FR: « Vous êtes sûr qu'elle ne l'a pas encore envoyée ? »

**85. `h4-046`** · ch2 ¶104.2 · `ok` · status `silver`

- EN: Your Majesty will, of course, stay in London for the present?”
- FR: Votre Majesté restera, bien entendu, à Londres pour le moment ? »

**86. `h4-047`** · ch2 ¶109.0 · `ok` · status `silver`

- EN: “You have carte blanche.”
- FR: « Vous avez carte blanche. »

**87. `h4-048`** · ch2 ¶115.0 · `ok` · status `silver`

- EN: Holmes scribbled a receipt upon a sheet of his note-book and handed it to him.
- FR: Holmes griffonna un reçu sur un feuillet de son carnet et le lui tendit.

**88. `h4-049`** · ch2 ¶118.2 · `ok` · status `silver`

- EN: “Was the photograph a cabinet?”
- FR: « La photographie était-elle au format cabinet ? »

**89. `h4-050`** · ch2 ¶122.0 · `ok` · status `silver`

- EN: At three o’clock precisely I was at Baker Street, but Holmes had not yet returned.
- FR: À trois heures précises, j’étais à Baker Street, mais Holmes n’était pas encore rentré.

**90. `h4-051`** · ch2 ¶122.5 · `ok` · status `silver`

- EN: So accustomed was I to his invariable success that the very possibility of his failing had ceased to enter into my head.
- FR: J’étais si accoutumé à son invariable succès que la possibilité même d’un échec de sa part avait cessé d’entrer dans mon esprit.

**91. `h4-055`** · ch2 ¶129.0 · `ok` · status `silver`

- EN: “I then lounged down the street and found, as I expected, that there was a mews in a lane which runs down by one wall of the garden.
- FR: « J’ai alors flâné dans la rue et j’ai constaté, comme je m’y attendais, qu’il y avait une allée d’écuries dans une ruelle qui longe l’un des murs du jardin.

**92. `h4-056`** · ch2 ¶131.1 · `ok` · status `silver`

- EN: She is the daintiest thing under a bonnet on this planet.
- FR: C’est la plus ravissante créature sous un chapeau sur cette planète.

**93. `h4-057`** · ch2 ¶131.6 · `ok` · status `silver`

- EN: He is dark, handsome, and dashing, never calls less than once a day, and often twice.
- FR: Il est brun, bel homme et fringant, ne passe jamais moins d’une fois par jour, et souvent deux fois.

**94. `h4-058`** · ch2 ¶132.0 · `ok` · status `silver`

- EN: “This Godfrey Norton was evidently an important factor in the matter.
- FR: « Ce Godfrey Norton était manifestement un facteur important dans cette affaire.

**95. `h4-059`** · ch2 ¶132.6 · `ok` · status `silver`

- EN: If the latter, it was less likely.
- FR: Si c'était la dernière, c'était moins probable.

**96. `h4-060`** · ch2 ¶134.0 · `ok` · status `silver`

- EN: “I was still balancing the matter in my mind when a hansom cab drove up to Briony Lodge, and a gentleman sprang out.
- FR: « J'étais encore en train de peser le pour et le contre dans mon esprit lorsqu'un cab hansom arriva devant Briony Lodge, et un monsieur en jaillit.

**97. `h4-061`** · ch2 ¶135.2 · `ok` · status `silver`

- EN: Presently he emerged, looking even more flurried than before.
- FR: Bientôt, il réapparut, l'air encore plus agité qu'auparavant.

**98. `h4-063`** · ch2 ¶138.2 · `ok` · status `silver`

- EN: The driver looked twice at such a shabby fare, but I jumped in before he could object.
- FR: Le cocher y regarda à deux fois devant un client si miteux, mais je sautai dedans avant qu’il ne pût protester.

**99. `h4-064`** · ch2 ¶139.2 · `ok` · status `silver`

- EN: The cab and the landau with their steaming horses were in front of the door when I arrived.
- FR: Le fiacre et la calèche avec leurs chevaux fumants étaient devant la porte quand je suis arrivé.

**100. `h4-065`** · ch2 ¶139.7 · `ok` · status `silver`

- EN: Suddenly, to my surprise, the three at the altar faced round to me, and Godfrey Norton came running as hard as he could towards me.
- FR: Soudain, à ma grande surprise, les trois personnes à l’autel se sont retournées vers moi, et Godfrey Norton s’est mis à courir de toutes ses forces dans ma direction.

**101. `h4-068`** · ch2 ¶145.5 · `ok` · status `silver`

- EN: They drove away in different directions, and I went off to make my own arrangements.”
- FR: Ils partirent en voiture dans des directions différentes, et je m'en allai prendre mes propres dispositions. »

**102. `h4-069`** · ch2 ¶149.0 · `ok` · status `silver`

- EN: “You don’t mind breaking the law?”
- FR: « Cela ne vous dérange pas d'enfreindre la loi ? »

**103. `h4-070`** · ch2 ¶154.0 · `ok` · status `silver`

- EN: “Then I am your man.”
- FR: « Alors je suis votre homme. »

**104. `h4-071`** · ch2 ¶157.2 · `ok` · status `silver`

- EN: It is nearly five now.
- FR: Il est presque cinq heures à présent.

**105. `h4-072`** · ch2 ¶159.1 · `ok` · status `silver`

- EN: There is only one point on which I must insist.
- FR: Il n'y a qu'un seul point sur lequel je dois insister.

**106. `h4-073`** · ch2 ¶161.2 · `ok` · status `silver`

- EN: Do not join in it.
- FR: Ne vous en mêlez pas.

**107. `h4-074`** · ch2 ¶163.0 · `ok` · status `silver`

- EN: “You are to watch me, for I will be visible to you.”
- FR: « Vous devez me surveiller, car je vous serai visible. »

**108. `h4-075`** · ch2 ¶167.2 · `ok` · status `silver`

- EN: Your task is confined to that.
- FR: Votre tâche se limite à cela.

**109. `h4-076`** · ch2 ¶170.0 · `ok` · status `silver`

- EN: “Then you may entirely rely on me.”
- FR: « Alors vous pouvez entièrement compter sur moi. »

**110. `h4-077`** · ch2 ¶172.3 · `ok` · status `silver`

- EN: His expression, his manner, his very soul seemed to vary with every fresh part that he assumed.
- FR: Son expression, ses manières, son âme même semblaient varier avec chaque nouveau rôle qu'il assumait.

**111. `h4-078`** · ch2 ¶173.3 · `ok` · status `silver`

- EN: On the contrary, for a small street in a quiet neighbourhood, it was remarkably animated.
- FR: Au contraire, pour une petite rue dans un quartier tranquille, elle était remarquablement animée.

**112. `h4-079`** · ch2 ¶174.3 · `ok` · status `silver`

- EN: Now the question is, Where are we to find the photograph?”
- FR: Maintenant, la question est : où devons-nous trouver la photographie ? »

**113. `h4-081`** · ch2 ¶178.2 · `ok` · status `silver`

- EN: But I am inclined to think neither.
- FR: Mais j'ai tendance à penser : ni l'un ni l'autre.

**114. `h4-084`** · ch2 ¶186.2 · `ok` · status `silver`

- EN: It is her carriage.
- FR: C'est sa voiture.

**115. `h4-085`** · ch2 ¶187.3 · `ok` · status `silver`

- EN: A fierce quarrel broke out, which was increased by the two guardsmen, who took sides with one of the loungers, and by the scissors-grinder, who was equally hot upon the other side.
- FR: Une violente dispute éclata, encore envenimée par les deux gardes, qui prirent parti pour l'un des traîneurs, et par le rémouleur, qui s'échauffait tout autant de l'autre côté.

**116. `h4-086`** · ch2 ¶187.7 · `ok` · status `silver`

- EN: Irene Adler, as I will still call her, had hurried up the steps; but she stood at the top with her superb figure outlined against the lights of the hall, looking back into the street.
- FR: Irene Adler, comme je continuerai de l'appeler, s'était dépêchée de monter les marches ; mais elle se tenait au sommet, sa silhouette superbe se découpant sur les lumières du vestibule, regardant en arrière vers la rue.

**117. `h4-088`** · ch2 ¶192.1 · `ok` · status `silver`

- EN: May we bring him in, marm?”
- FR: Pouvons-nous le faire entrer, m'ame ? »

**118. `h4-089`** · ch2 ¶194.2 · `ok` · status `silver`

- EN: I do not know whether he was seized with compunction at that moment for the part he was playing, but I know that I never felt more heartily ashamed of myself in my life than when I saw the beautiful creature against whom I was conspiring, or the grace and kindliness with which she waited upon the injured man.
- FR: J'ignore s'il fut pris de remords à cet instant pour le rôle qu'il jouait, mais je sais que je n'ai jamais eu plus profondément honte de moi-même de ma vie que lorsque je vis la belle créature contre laquelle je conspirais, ou la grâce et la bienveillance avec lesquelles elle soignait l'homme blessé.

**119. `h4-090`** · ch2 ¶195.1 · `ok` · status `silver`

- EN: A maid rushed across and threw open the window.
- FR: Une servante se précipita et ouvrit grand la fenêtre.

**120. `h4-091`** · ch2 ¶195.6 · `ok` · status `silver`

- EN: Slipping through the shouting crowd I made my way to the corner of the street, and in ten minutes was rejoiced to find my friend’s arm in mine, and to get away from the scene of uproar.
- FR: Me faufilant à travers la foule qui criait, je me frayai un chemin jusqu'au coin de la rue, et au bout de dix minutes je fus ravi de retrouver le bras de mon ami passé sous le mien, et de m'éloigner du lieu du tumulte.

**121. `h4-093`** · ch2 ¶201.0 · `ok` · status `silver`

- EN: “I am still in the dark.”
- FR: « Je suis toujours dans l'obscurité. »

**122. `h4-094`** · ch2 ¶203.0 · `ok` · status `silver`

- EN: “I guessed as much.”
- FR: « Je m'en doutais bien. »

**123. `h4-095`** · ch2 ¶206.0 · `ok` · status `silver`

- EN: “Then they carried me in.
- FR: « Ensuite, ils m'ont porté à l'intérieur.

**124. `h4-097`** · ch2 ¶208.4 · `ok` · status `silver`

- EN: A married woman grabs at her baby; an unmarried one reaches for her jewel-box.
- FR: Une femme mariée s'empare de son bébé ; une femme non mariée tend la main vers son coffret à bijoux.

**125. `h4-098`** · ch2 ¶208.8 · `ok` · status `silver`

- EN: The smoke and shouting were enough to shake nerves of steel.
- FR: La fumée et les cris étaient suffisants pour ébranler des nerfs d’acier.

**126. `h4-099`** · ch2 ¶208.14 · `ok` · status `silver`

- EN: I hesitated whether to attempt to secure the photograph at once; but the coachman had come in, and as he was watching me narrowly, it seemed safer to wait.
- FR: J’ai hésité à tenter de m’emparer de la photographie immédiatement ; mais le cocher était entré, et comme il me surveillait étroitement, il a semblé plus sûr d’attendre.

**127. `h4-100`** · ch2 ¶210.3 · `ok` · status `silver`

- EN: It might be a satisfaction to his Majesty to regain it with his own hands.”
- FR: Ce pourrait être une satisfaction pour Sa Majesté de la récupérer de ses propres mains. »

**128. `h4-101`** · ch2 ¶212.3 · `ok` · status `silver`

- EN: I must wire to the King without delay.”
- FR: Je dois télégraphier au roi sans délai. »

**129. `h4-102`** · ch2 ¶216.0 · `ok` · status `silver`

- EN: “I’ve heard that voice before,” said Holmes, staring down the dimly lit street.
- FR: « J'ai déjà entendu cette voix-là », dit Holmes, fixant la rue faiblement éclairée.

**130. `h4-103`** · ch2 ¶223.1 · `ok` · status `silver`

- EN: I am all impatience to be gone.”
- FR: Je brûle d'impatience de partir. »

**131. `h4-104`** · ch2 ¶226.1 · `ok` · status `silver`

- EN: We descended and started off once more for Briony Lodge.
- FR: Nous descendîmes et nous mîmes en route une fois de plus pour Briony Lodge.

**132. `h4-106`** · ch2 ¶236.4 · `ok` · status `silver`

- EN: What a queen she would have made!”
- FR: Quelle reine elle aurait faite ! »

**133. `h4-107`** · ch2 ¶239.0 · `ok` · status `silver`

- EN: “I am Mr. Holmes,” answered my companion, looking at her with a questioning and rather startled gaze.
- FR: « Je suis M. Holmes », répondit mon compagnon en la regardant d’un air interrogateur et assez surpris.

**134. `h4-108`** · ch2 ¶243.0 · `ok` · status `silver`

- EN: “And the papers?” asked the King hoarsely.
- FR: « Et les papiers ? demanda le roi d'une voix rauque.

**135. `h4-109`** · ch2 ¶244.5 · `ok` · status `silver`

- EN: To be left till called for.”
- FR: Poste restante. »

**136. `h4-110`** · ch2 ¶245.1 · `ok` · status `silver`

- EN: You took me in completely.
- FR: Vous m'avez complètement trompée.

**137. `h4-111`** · ch2 ¶245.6 · `ok` · status `silver`

- EN: And your address had been given me.
- FR: Et votre adresse m'avait été donnée.

**138. `h4-112`** · ch2 ¶245.11 · `ok` · status `silver`

- EN: I sent John, the coachman, to watch you, ran upstairs, got into my walking clothes, as I call them, and came down just as you departed.
- FR: J'ai envoyé John, le cocher, pour vous surveiller, j'ai couru à l'étage, j'ai enfilé mes vêtements de marche, comme je les appelle, et je suis descendue juste au moment où vous partiez.

**139. `h4-113`** · ch2 ¶245.16 · `ok` · status `silver`

- EN: I love and am loved by a better man than he.
- FR: J'aime et je suis aimée par un homme meilleur que lui.

**140. `h4-114`** · ch2 ¶247.0 · `ok` · status `silver`

- EN: “What a woman—oh, what a woman!” cried the King of Bohemia, when we had all three read this epistle.
- FR: « Quelle femme — oh, quelle femme ! » s'écria le roi de Bohême, quand nous eûmes tous les trois lu cette épître.

**141. `h4-115`** · ch2 ¶248.1 · `ok` · status `silver`

- EN: “I am sorry that I have not been able to bring your Majesty’s business to a more successful conclusion.”
- FR: « Je suis désolé de n'avoir pas été capable de mener l'affaire de votre Majesté à une conclusion plus heureuse. »

**142. `h4-116`** · ch2 ¶250.0 · `ok` · status `silver`

- EN: “I am glad to hear your Majesty say so.”
- FR: « Je suis heureux d'entendre votre Majesté le dire. »

**143. `h4-117`** · ch2 ¶253.0 · `ok` · status `silver`

- EN: “You have but to name it.”
- FR: « Vous n'avez qu'à le nommer. »

**144. `h4-118`** · ch2 ¶257.1 · `ok` · status `silver`

- EN: Then there is no more to be done in the matter.
- FR: Alors il n'y a plus rien à faire dans cette affaire.

**145. `h4-119`** · ch2 ¶258.2 · `ok` · status `silver`

- EN: And when he speaks of Irene Adler, or when he refers to her photograph, it is always under the honourable title of the woman.
- FR: Et lorsqu'il parle d'Irene Adler, ou lorsqu'il fait référence à sa photographie, c'est toujours sous le titre honorable de la femme.

**146. `h4-120`** · ch3 ¶5.0 · `ok` · status `silver`

- EN: “Then I can wait in the next room.”
- FR: « Alors je peux attendre dans la pièce d'à côté. »

**147. `h4-122`** · ch3 ¶12.0 · `ok` · status `silver`

- EN: “You did, Doctor, but none the less you must come round to my view, for otherwise I shall keep on piling fact upon fact on you until your reason breaks down under them and acknowledges me to be right.
- FR: « En effet, docteur, mais néanmoins vous devez vous ranger à mon avis, car sinon je continuerai à empiler fait sur fait devant vous jusqu'à ce que votre raison cède sous leur poids et reconnaisse que j'ai raison.

**148. `h4-123`** · ch3 ¶12.5 · `ok` · status `silver`

- EN: I ask you not merely because my friend Dr. Watson has not heard the opening part but also because the peculiar nature of the story makes me anxious to have every possible detail from your lips.
- FR: Je vous le demande non seulement parce que mon ami le docteur Watson n'a pas entendu le début, mais aussi parce que la nature particulière de cette histoire me rend désireux d'en recueillir chaque détail possible de vos lèvres.

**149. `h4-125`** · ch3 ¶15.0 · `ok` · status `silver`

- EN: Sherlock Holmes’ quick eye took in my occupation, and he shook his head with a smile as he noticed my questioning glances.
- FR: Le regard vif de Sherlock Holmes saisit mon occupation, et il secoua la tête avec un sourire en remarquant mes coups d'œil interrogateurs.

**150. `h4-126`** · ch3 ¶17.2 · `ok` · status `silver`

- EN: It’s as true as gospel, for I began as a ship’s carpenter.”
- FR: C’est aussi vrai que l’Évangile, car j’ai commencé comme charpentier de marine. »

**151. `h4-128`** · ch3 ¶24.1 · `ok` · status `silver`

- EN: I have made a small study of tattoo marks and have even contributed to the literature of the subject.
- FR: J’ai fait une petite étude des marques de tatouage et ai même contribué à la littérature sur le sujet.

**152. `h4-129`** · ch3 ¶25.2 · `ok` · status `silver`

- EN: “I thought at first that you had done something clever, but I see that there was nothing in it after all.”
- FR: « J'avais d'abord pensé que vous aviez fait quelque chose d'ingénieux, mais je vois qu'il n'y avait rien là-dessous après tout. »

**153. `h4-130`** · ch3 ¶27.2 · `ok` · status `silver`

- EN: This is what began it all.
- FR: C'est cela qui a tout déclenché.

**154. `h4-131`** · ch3 ¶29.2 · `ok` · status `silver`

- EN: All red-headed men who are sound in body and mind and above the age of twenty-one years, are eligible.
- FR: Tous les hommes roux qui sont sains de corps et d'esprit, et âgés de plus de vingt et un ans, sont éligibles.

**155. `h4-132`** · ch3 ¶31.1 · `ok` · status `silver`

- EN: “It is a little off the beaten track, isn’t it?” said he.
- FR: « C'est un peu hors des sentiers battus, n'est-ce pas ? » dit-il.

**156. `h4-133`** · ch3 ¶34.0 · `ok` · status `silver`

- EN: “Well, it is just as I have been telling you, Mr. Sherlock Holmes,” said Jabez Wilson, mopping his forehead; “I have a small pawnbroker’s business at Coburg Square, near the City.
- FR: « Eh bien, c'est exactement comme je vous le disais, monsieur Sherlock Holmes », dit Jabez Wilson en s'épongeant le front ; « j'ai un petit commerce de prêteur sur gages à Coburg Square, près de la City.

**157. `h4-134`** · ch3 ¶36.0 · `ok` · status `silver`

- EN: “His name is Vincent Spaulding, and he’s not such a youth, either.
- FR: « Il s'appelle Vincent Spaulding, et ce n'est pas tellement un jeune homme non plus.

**158. `h4-135`** · ch3 ¶37.2 · `ok` · status `silver`

- EN: It is not a common experience among employers in this age.
- FR: Ce n’est pas une expérience courante parmi les employeurs de notre époque.

**159. `h4-136`** · ch3 ¶38.3 · `ok` · status `silver`

- EN: That is his main fault, but on the whole he’s a good worker.
- FR: C’est là son principal défaut, mais dans l’ensemble, c’est un bon travailleur.

**160. `h4-137`** · ch3 ¶41.0 · `ok` · status `silver`

- EN: “The first thing that put us out was that advertisement.
- FR: « La première chose qui nous a dérangés, ce fut cette annonce.

**161. `h4-138`** · ch3 ¶44.2 · `ok` · status `silver`

- EN: If my hair would only change colour, here’s a nice little crib all ready for me to step into.
- FR: Si seulement mes cheveux changeaient de couleur, voilà une gentille petite place toute prête pour que je m'y installe.

**162. `h4-139`** · ch3 ¶46.1 · `ok` · status `silver`

- EN: ’ he asked with his eyes open.
- FR: ” demanda-t-il en ouvrant de grands yeux.

**163. `h4-141`** · ch3 ¶53.3 · `ok` · status `silver`

- EN: From all I hear it is splendid pay and very little to do.’
- FR: D'après tout ce que j'entends dire, c'est une paye splendide et très peu de travail.”

**164. `h4-142`** · ch3 ¶55.3 · `ok` · status `silver`

- EN: Then, again, I have heard it is no use your applying if your hair is light red, or dark red, or anything but real bright, blazing, fiery red.
- FR: Et puis, d'un autre côté, j'ai entendu dire qu'il est inutile de postuler si vos cheveux sont d'un roux clair, ou d'un roux foncé, ou de toute autre couleur qu'un véritable roux vif, flamboyant, ardent.

**165. `h4-143`** · ch3 ¶57.0 · `ok` · status `silver`

- EN: “I never hope to see such a sight as that again, Mr. Holmes.
- FR: « Je n'espère plus jamais voir un tel spectacle, monsieur Holmes.

**166. `h4-144`** · ch3 ¶57.5 · `ok` · status `silver`

- EN: When I saw how many were waiting, I would have given it up in despair; but Spaulding would not hear of it.
- FR: Quand je vis combien attendaient, j'aurais bien renoncé de désespoir ; mais Spaulding ne voulut rien entendre.

**167. `h4-145`** · ch3 ¶59.0 · `ok` · status `silver`

- EN: “There was nothing in the office but a couple of wooden chairs and a deal table, behind which sat a small man with a head that was even redder than mine.
- FR: « Il n'y avait rien dans le bureau hormis une paire de chaises en bois et une table en sapin, derrière laquelle était assis un petit homme avec une tête encore plus rousse que la mienne.

**168. `h4-147`** · ch3 ¶61.4 · `ok` · status `silver`

- EN: Then suddenly he plunged forward, wrung my hand, and congratulated me warmly on my success.
- FR: Puis, soudain, il s'élança en avant, me serra la main, et me félicita chaleureusement pour mon succès.

**169. `h4-148`** · ch3 ¶62.4 · `ok` · status `silver`

- EN: ‘I perceive that all is as it should be.
- FR: “Je constate que tout est comme il se doit.

**170. `h4-149`** · ch3 ¶63.0 · `ok` · status `silver`

- EN: “‘My name,’ said he, ‘is Mr. Duncan Ross, and I am myself one of the pensioners upon the fund left by our noble benefactor.
- FR: « “Mon nom”, dit-il, “est M. Duncan Ross, et je suis moi-même l'un des pensionnaires du fonds légué par notre noble bienfaiteur.

**171. `h4-150`** · ch3 ¶66.1 · `ok` · status `silver`

- EN: ’ he said gravely, ‘that is very serious indeed!
- FR: dit-il gravement, « voilà qui est très sérieux, en vérité !

**172. `h4-151`** · ch3 ¶68.0 · `ok` · status `silver`

- EN: “‘In the case of another,’ said he, ‘the objection might be fatal, but we must stretch a point in favour of a man with such a head of hair as yours.
- FR: « “Dans le cas d'un autre”, dit-il, “l'objection pourrait être fatale, mais nous devons faire une exception en faveur d'un homme doté d'une telle chevelure que la vôtre.

**173. `h4-152`** · ch3 ¶70.1 · `ok` · status `silver`

- EN: ’ said Vincent Spaulding.
- FR: ” dit Vincent Spaulding.

**174. `h4-153`** · ch3 ¶74.0 · `ok` · status `silver`

- EN: “‘That would suit me very well,’ said I. ‘And the pay?’
- FR: « “Cela me conviendrait très bien”, dis-je. “Et la rémunération ?”

**175. `h4-154`** · ch3 ¶79.2 · `ok` · status `silver`

- EN: The will is very clear upon that point.
- FR: Le testament est très clair sur ce point.

**176. `h4-155`** · ch3 ¶83.0 · `ok` · status `silver`

- EN: “‘Is to copy out the Encyclopædia Britannica.
- FR: « Est de recopier l’Encyclopædia Britannica.

**177. `h4-157`** · ch3 ¶87.0 · `ok` · status `silver`

- EN: “Well, to my surprise and delight, everything was as right as possible.
- FR: « Eh bien, à ma grande surprise et à mon grand ravissement, tout était aussi parfait que possible.

**178. `h4-159`** · ch3 ¶89.0 · `ok` · status `silver`

- EN: “Eight weeks passed away like this, and I had written about Abbots and Archery and Armour and Architecture and Attica, and hoped with diligence that I might get on to the B’s before very long.
- FR: « Huit semaines s'écoulèrent ainsi, et j'avais écrit sur les Abbés, le Tir à l'arc, les Armures, l'Architecture et l'Attique, et j'espérais avec assiduité pouvoir passer aux B avant très longtemps.

**179. `h4-161`** · ch3 ¶95.0 · `ok` · status `silver`

- EN: “I cannot see that there is anything very funny,” cried our client, flushing up to the roots of his flaming head.
- FR: « Je ne vois pas qu’il y ait quoi que ce soit de très drôle », s’écria notre client, rougissant jusqu’à la racine de sa tête flamboyante.

**180. `h4-162`** · ch3 ¶96.3 · `ok` · status `silver`

- EN: But there is, if you will excuse my saying so, something just a little funny about it.
- FR: Mais il y a, si vous me permettez de le dire, quelque chose d'un tout petit peu drôle là-dedans.

**181. `h4-163`** · ch3 ¶97.3 · `ok` · status `silver`

- EN: Finally, I went to the landlord, who is an accountant living on the ground floor, and I asked him if he could tell me what had become of the Red-headed League.
- FR: Finalement, je suis allé voir le propriétaire, qui est un comptable habitant au rez-de-chaussée, et je lui ai demandé s'il pouvait me dire ce qu'était devenue la Ligue des roux.

**182. `h4-164`** · ch3 ¶98.0 · `ok` · status `silver`

- EN: “‘Well,’ said I, ‘the gentleman at No. 4.’
- FR: « “Eh bien,” dis-je, “le monsieur du numéro 4.”

**183. `h4-165`** · ch3 ¶102.0 · `ok` · status `silver`

- EN: “‘Where could I find him?
- FR: « “Où pourrais-je le trouver ?”

**184. `h4-166`** · ch3 ¶105.0 · `ok` · status `silver`

- EN: “And what did you do then?” asked Holmes.
- FR: « Et qu’avez-vous fait ensuite ? » demanda Holmes.

**185. `h4-167`** · ch3 ¶106.4 · `ok` · status `silver`

- EN: I did not wish to lose such a place without a struggle, so, as I had heard that you were good enough to give advice to poor folk who were in need of it, I came right away to you.”
- FR: Je ne voulais pas perdre une telle place sans lutter, alors, comme j’avais entendu dire que vous aviez la bonté de donner des conseils aux pauvres gens qui en avaient besoin, je suis venu immédiatement à vous. »

**186. `h4-168`** · ch3 ¶108.1 · `ok` · status `silver`

- EN: “Why, I have lost four pound a week.”
- FR: « Eh bien, j'ai perdu quatre livres par semaine. »

**187. `h4-169`** · ch3 ¶111.0 · `ok` · status `silver`

- EN: “We shall endeavour to clear up these points for you.
- FR: « Nous nous efforcerons d'éclaircir ces points pour vous.

**188. `h4-170`** · ch3 ¶113.0 · `ok` · status `silver`

- EN: “How did he come?”
- FR: « Comment est-il venu ? »

**189. `h4-171`** · ch3 ¶118.0 · `ok` · status `silver`

- EN: “Because he was handy and would come cheap.”
- FR: « Parce qu'il était habile et ne demandait pas cher. »

**190. `h4-172`** · ch3 ¶123.0 · `ok` · status `silver`

- EN: Holmes sat up in his chair in considerable excitement.
- FR: Holmes se redressa sur sa chaise avec une vive excitation.

**191. `h4-173`** · ch3 ¶125.1 · `ok` · status `silver`

- EN: “He is still with you?”
- FR: « Il est toujours avec vous ? »

**192. `h4-174`** · ch3 ¶129.0 · `ok` · status `silver`

- EN: “That will do, Mr. Wilson.
- FR: « Cela suffira, monsieur Wilson.

**193. `h4-175`** · ch3 ¶131.1 · `ok` · status `silver`

- EN: “It is a most mysterious business.”
- FR: C’est une affaire des plus mystérieuses. »

**194. `h4-176`** · ch3 ¶133.0 · `ok` · status `silver`

- EN: “What are you going to do, then?”
- FR: « Qu’allez-vous faire, alors ? »

**195. `h4-177`** · ch3 ¶135.0 · `ok` · status `silver`

- EN: “Sarasate plays at the St. James’s Hall this afternoon,” he remarked.
- FR: « Sarasate joue au St. James’s Hall cet après-midi, remarqua-t-il.

**196. `h4-178`** · ch3 ¶137.0 · `ok` · status `silver`

- EN: “Then put on your hat and come.
- FR: « Alors mettez votre chapeau et venez.

**197. `h4-180`** · ch3 ¶138.6 · `ok` · status `silver`

- EN: It was instantly opened by a bright-looking, clean-shaven young fellow, who asked him to step in.
- FR: Elle fut instantanément ouverte par un jeune homme d’allure vive, rasé de près, qui le pria d’entrer.

**198. `h4-182`** · ch3 ¶146.0 · `ok` · status `silver`

- EN: “And what did you see?”
- FR: « Et qu'avez-vous vu ? »

**199. `h4-184`** · ch3 ¶150.3 · `ok` · status `silver`

- EN: It was difficult to realise as we looked at the line of fine shops and stately business premises that they really abutted on the other side upon the faded and stagnant square which we had just quitted.
- FR: Il était difficile de réaliser, tandis que nous regardions la rangée de beaux magasins et d'imposants locaux commerciaux, qu'ils donnaient réellement de l'autre côté sur le square défraîchi et stagnant que nous venions tout juste de quitter.

**200. `h4-185`** · ch3 ¶151.4 · `ok` · status `silver`

- EN: And now, Doctor, we’ve done our work, so it’s time we had some play.
- FR: Et maintenant, docteur, nous avons fini notre travail, il est donc temps que nous nous divertissions un peu.

**201. `h4-186`** · ch3 ¶152.3 · `ok` · status `silver`

- EN: The swing of his nature took him from extreme languor to devouring energy; and, as I knew well, he was never so truly formidable as when, for days on end, he had been lounging in his armchair amid his improvisations and his black-letter editions.
- FR: Le balancement de sa nature le portait d'une langueur extrême à une énergie dévorante ; et, comme je le savais bien, il n'était jamais aussi véritablement redoutable que lorsque, des jours d'affilée, il s'était prélassé dans son fauteuil au milieu de ses improvisations et de ses éditions en caractères gothiques.

**202. `h4-187`** · ch3 ¶155.0 · `ok` · status `silver`

- EN: “And I have some business to do which will take some hours.
- FR: « Et j'ai quelques affaires à régler qui prendront plusieurs heures.

**203. `h4-188`** · ch3 ¶157.2 · `ok` · status `silver`

- EN: But to-day being Saturday rather complicates matters.
- FR: Mais le fait qu'aujourd'hui soit samedi complique plutôt les choses.

**204. `h4-189`** · ch3 ¶161.2 · `ok` · status `silver`

- EN: He waved his hand, turned on his heel, and disappeared in an instant among the crowd.
- FR: Il fit un geste de la main, tourna sur ses talons, et disparut en un instant dans la foule.

**205. `h4-190`** · ch3 ¶162.4 · `ok` · status `silver`

- EN: Where were we going, and what were we to do?
- FR: Où allions-nous, et qu'étions-nous censés faire ?

**206. `h4-192`** · ch3 ¶165.1 · `ok` · status `silver`

- EN: “Our friend here is a wonderful man for starting a chase.
- FR: « Notre ami ici présent est un homme merveilleux pour lancer une poursuite.

**207. `h4-193`** · ch3 ¶167.2 · `ok` · status `silver`

- EN: It is not too much to say that once or twice, as in that business of the Sholto murder and the Agra treasure, he has been more nearly correct than the official force.”
- FR: Il n'est pas exagéré de dire qu'une ou deux fois, comme dans cette affaire du meurtre de Sholto et du trésor d'Agra, il a été plus près de la vérité que la police officielle. »

**208. `h4-194`** · ch3 ¶169.0 · `ok` · status `silver`

- EN: “I think you will find,” said Sherlock Holmes, “that you will play for a higher stake to-night than you have ever done yet, and that the play will be more exciting.
- FR: « Je pense que vous découvrirez », dit Sherlock Holmes, « que vous jouerez ce soir pour un enjeu plus élevé que vous ne l'avez jamais fait jusqu'ici, et que la partie sera plus passionnante.

**209. `h4-195`** · ch3 ¶170.3 · `ok` · status `silver`

- EN: His grandfather was a royal duke, and he himself has been to Eton and Oxford.
- FR: Son grand-père était un duc royal, et lui-même a fréquenté Eton et Oxford.

**210. `h4-196`** · ch3 ¶171.1 · `ok` · status `silver`

- EN: I’ve had one or two little turns also with Mr. John Clay, and I agree with you that he is at the head of his profession.
- FR: J'ai eu un ou deux petits démêlés moi aussi avec M. John Clay, et je conviens avec vous qu'il est au sommet de sa profession.

**211. `h4-197`** · ch3 ¶173.0 · `ok` · status `silver`

- EN: “We are close there now,” my friend remarked.
- FR: « Nous y sommes presque maintenant », fit remarquer mon ami.

**212. `h4-198`** · ch3 ¶173.5 · `ok` · status `silver`

- EN: He is as brave as a bulldog and as tenacious as a lobster if he gets his claws upon anyone.
- FR: Il est aussi courageux qu'un bouledogue et aussi tenace qu'un homard s'il met ses pinces sur quelqu'un.

**213. `h4-200`** · ch3 ¶176.1 · `ok` · status `silver`

- EN: “Why, dear me, it sounds quite hollow!” he remarked, looking up in surprise.
- FR: « Mais ma parole, cela sonne tout à fait creux ! » remarqua-t-il en levant les yeux de surprise.

**214. `h4-201`** · ch3 ¶178.1 · `ok` · status `silver`

- EN: A few seconds sufficed to satisfy him, for he sprang to his feet again and put his glass in his pocket.
- FR: Quelques secondes suffirent à le satisfaire, car il se remit aussitôt sur pied et mit sa loupe dans sa poche.

**215. `h4-202`** · ch3 ¶180.0 · `ok` · status `silver`

- EN: “It is our French gold,” whispered the director.
- FR: « C'est notre or français », chuchota le directeur.

**216. `h4-203`** · ch3 ¶182.4 · `ok` · status `silver`

- EN: Our reserve of bullion is much larger at present than is usually kept in a single branch office, and the directors have had misgivings upon the subject.”
- FR: Notre réserve de lingots est bien plus importante à présent que ce qui est habituellement conservé dans une simple succursale, et les directeurs ont eu des appréhensions à ce sujet. »

**217. `h4-204`** · ch3 ¶184.0 · `ok` · status `silver`

- EN: “And sit in the dark?”
- FR: « Et rester assis dans le noir ? »

**218. `h4-205`** · ch3 ¶185.4 · `ok` · status `silver`

- EN: These are daring men, and though we shall take them at a disadvantage, they may do us some harm unless we are careful.
- FR: Ce sont des hommes audacieux, et bien que nous allions les prendre au dépourvu, ils peuvent nous faire du tort si nous ne prenons pas garde.

**219. `h4-206`** · ch3 ¶186.0 · `ok` · status `silver`

- EN: I placed my revolver, cocked, upon the top of the wooden case behind which I crouched.
- FR: Je posai mon revolver, armé, sur le dessus de la caisse en bois derrière laquelle j'étais accroupi.

**220. `h4-207`** · ch3 ¶187.1 · `ok` · status `silver`

- EN: “That is back through the house into Saxe-Coburg Square.
- FR: C'est de retourner par la maison pour gagner le square Saxe-Cobourg.

**221. `h4-208`** · ch3 ¶190.0 · `ok` · status `silver`

- EN: What a time it seemed!
- FR: Que le temps parut long !

**222. `h4-209`** · ch3 ¶191.0 · `ok` · status `silver`

- EN: At first it was but a lurid spark upon the stone pavement.
- FR: Au début, ce ne fut qu'une étincelle blafarde sur le pavé de pierre.

**223. `h4-210`** · ch3 ¶192.1 · `ok` · status `silver`

- EN: With a rending, tearing sound, one of the broad, white stones turned over upon its side and left a square, gaping hole, through which streamed the light of a lantern.
- FR: Avec un bruit de déchirement et d'arrachement, l'une des larges pierres blanches bascula sur le côté et laissa un trou carré et béant, à travers lequel se déversait la lumière d'une lanterne.

**224. `h4-211`** · ch3 ¶193.3 · `ok` · status `silver`

- EN: Jump, Archie, jump, and I’ll swing for it!”
- FR: Saute, Archie, saute, et je serai pendu pour ça ! »

**225. `h4-212`** · ch3 ¶195.0 · `ok` · status `silver`

- EN: “It’s no use, John Clay,” said Holmes blandly.
- FR: « C'est inutile, John Clay », dit Holmes d'un ton suave.

**226. `h4-213`** · ch3 ¶198.1 · `ok` · status `silver`

- EN: You seem to have done the thing very completely.
- FR: Vous semblez avoir fait les choses de façon très complète.

**227. `h4-214`** · ch3 ¶200.1 · `ok` · status `silver`

- EN: “He’s quicker at climbing down holes than I am.
- FR: « Il est plus prompt que moi à descendre dans les trous.

**228. `h4-215`** · ch3 ¶202.0 · `ok` · status `silver`

- EN: “All right,” said Jones with a stare and a snigger.
- FR: « Très bien », dit Jones avec un regard fixe et un ricanement.

**229. `h4-216`** · ch3 ¶204.1 · `ok` · status `silver`

- EN: There is no doubt that you have detected and defeated in the most complete manner one of the most determined attempts at bank robbery that have ever come within my experience.”
- FR: Il n'y a aucun doute que vous avez découvert et déjoué de la manière la plus complète l'une des tentatives de cambriolage de banque les plus résolues qui se soient jamais présentées dans mon expérience. »

**230. `h4-217`** · ch3 ¶206.2 · `ok` · status `silver`

- EN: The method was no doubt suggested to Clay’s ingenious mind by the colour of his accomplice’s hair.
- FR: La méthode fut sans doute suggérée à l'esprit ingénieux de Clay par la couleur des cheveux de son complice.

**231. `h4-218`** · ch3 ¶207.0 · `ok` · status `silver`

- EN: “But how could you guess what the motive was?”
- FR: « Mais comment pouviez-vous deviner quel était ce motif ? »

**232. `h4-219`** · ch3 ¶208.4 · `ok` · status `silver`

- EN: What could it be?
- FR: Qu'est-ce que cela pouvait être ?

**233. `h4-220`** · ch3 ¶208.10 · `ok` · status `silver`

- EN: What could it be, once more?
- FR: Qu'est-ce que cela pouvait être, encore une fois ?

**234. `h4-221`** · ch3 ¶209.3 · `ok` · status `silver`

- EN: It was not in front.
- FR: Elle n'était pas devant.

**235. `h4-222`** · ch3 ¶209.8 · `ok` · status `silver`

- EN: You must yourself have remarked how worn, wrinkled, and stained they were.
- FR: Vous avez dû vous-même remarquer combien ils étaient usés, froissés et tachés.

**236. `h4-223`** · ch3 ¶210.0 · `ok` · status `silver`

- EN: “And how could you tell that they would make their attempt to-night?”
- FR: « Et comment pouviez-vous savoir qu'ils feraient leur tentative ce soir ? »

**237. `h4-224`** · ch3 ¶211.3 · `ok` · status `silver`

- EN: For all these reasons I expected them to come to-night.”
- FR: Pour toutes ces raisons, je m'attendais à ce qu'ils viennent ce soir. »

**238. `h4-225`** · ch3 ¶213.3 · `ok` · status `silver`

- EN: My life is spent in one long effort to escape from the commonplaces of existence.
- FR: Ma vie se passe en un long effort pour échapper aux lieux communs de l'existence.

**239. `h4-226`** · ch3 ¶215.2 · `ok` · status `silver`

- EN: “‘L’homme c’est rien—l’œuvre c’est tout,’ as Gustave Flaubert wrote to George Sand.”
- FR: « “L’homme c’est rien — l’œuvre c’est tout”, comme Gustave Flaubert l'écrivit à George Sand. »

**240. `h4-227`** · ch4 ¶2.0 · `ok` · status `silver`

- EN: “And yet I am not convinced of it,” I answered.
- FR: « Et pourtant je n'en suis pas convaincu », répondis-je.

**241. `h4-228`** · ch4 ¶3.2 · `ok` · status `silver`

- EN: Depend upon it, there is nothing so unnatural as the commonplace.”
- FR: Soyez-en sûr, il n'y a rien d'aussi peu naturel que le lieu commun. »

**242. `h4-229`** · ch4 ¶4.4 · `ok` · status `silver`

- EN: Here is the first heading upon which I come.
- FR: Voici le premier titre sur lequel je tombe.

**243. `h4-231`** · ch4 ¶6.0 · `ok` · status `silver`

- EN: He held out his snuffbox of old gold, with a great amethyst in the centre of the lid.
- FR: Il me tendit sa tabatière de vieil or, avec une grande améthyste au centre du couvercle.

**244. `h4-232`** · ch4 ¶9.0 · `ok` · status `silver`

- EN: “It was from the reigning family of Holland, though the matter in which I served them was of such delicacy that I cannot confide it even to you, who have been good enough to chronicle one or two of my little problems.”
- FR: « Elle venait de la famille régnante de Hollande, bien que l'affaire dans laquelle je les ai servis fût d'une telle délicatesse que je ne peux la confier même à vous, qui avez eu l'amabilité de relater un ou deux de mes petits problèmes. »

**245. `h4-233`** · ch4 ¶11.2 · `ok` · status `silver`

- EN: Indeed, I have found that it is usually in unimportant matters that there is a field for the observation, and for the quick analysis of cause and effect which gives the charm to an investigation.
- FR: De fait, j'ai constaté que c'est d'ordinaire dans les affaires sans importance qu'il y a un champ pour l'observation, et pour l'analyse rapide de la cause et de l'effet qui donne tout son charme à une enquête.

**246. `h4-235`** · ch4 ¶13.2 · `ok` · status `silver`

- EN: She would like advice, but is not sure that the matter is not too delicate for communication.
- FR: Elle aimerait un conseil, mais n’est pas sûre que l’affaire ne soit pas trop délicate pour être communiquée.

**247. `h4-236`** · ch4 ¶13.6 · `ok` · status `silver`

- EN: But here she comes in person to resolve our doubts.”
- FR: Mais la voici qui vient en personne dissiper nos doutes. »

**248. `h4-237`** · ch4 ¶16.1 · `ok` · status `silver`

- EN: Then, suddenly realising the full purport of his words, she gave a violent start and looked up, with fear and astonishment upon her broad, good-humoured face.
- FR: Puis, réalisant soudain toute la portée de ses paroles, elle eut un violent sursaut et leva les yeux, la peur et l’étonnement peints sur son large visage débonnaire.

**249. `h4-238`** · ch4 ¶18.0 · `ok` · status `silver`

- EN: “I came to you, sir, because I heard of you from Mrs. Etherege, whose husband you found so easy when the police and everyone had given him up for dead.
- FR: « Je suis venue à vous, monsieur, parce que j'ai entendu parler de vous par Mme Etherege, dont vous avez retrouvé le mari si facilement alors que la police et tout le monde le croyaient mort.

**250. `h4-239`** · ch4 ¶20.1 · `ok` · status `silver`

- EN: “Yes, I did bang out of the house,” she said, “for it made me angry to see the easy way in which Mr. Windibank—that is, my father—took it all.
- FR: « Oui, je suis partie en claquant la porte », dit-elle, « car cela m'a mise en colère de voir avec quelle désinvolture M. Windibank — c'est-à-dire mon père — prenait tout cela.

**251. `h4-241`** · ch4 ¶26.0 · `ok` · status `silver`

- EN: “Your own little income,” he asked, “does it come out of the business?”
- FR: « Votre propre petit revenu, demanda-t-il, provient-il du commerce ? »

**252. `h4-242`** · ch4 ¶28.0 · `ok` · status `silver`

- EN: “You interest me extremely,” said Holmes.
- FR: « Vous m’intéressez au plus haut point », dit Holmes.

**253. `h4-243`** · ch4 ¶29.2 · `ok` · status `silver`

- EN: Mr. Windibank draws my interest every quarter and pays it over to mother, and I find that I can do pretty well with what I earn at typewriting.
- FR: M. Windibank touche mes intérêts chaque trimestre et les remet à ma mère, et je trouve que je me débrouille plutôt bien avec ce que je gagne à la dactylographie.

**254. `h4-244`** · ch4 ¶31.0 · `ok` · status `silver`

- EN: A flush stole over Miss Sutherland’s face, and she picked nervously at the fringe of her jacket.
- FR: Une rougeur envahit le visage de miss Sutherland, et elle tripota nerveusement la frange de sa veste.

**255. `h4-245`** · ch4 ¶31.5 · `ok` · status `silver`

- EN: He would get quite mad if I wanted so much as to join a Sunday-school treat.
- FR: Il devenait tout à fait furieux si je voulais ne serait-ce que participer à une fête de l'école du dimanche.

**256. `h4-246`** · ch4 ¶32.0 · `ok` · status `silver`

- EN: “I suppose,” said Holmes, “that when Mr. Windibank came back from France he was very annoyed at your having gone to the ball.”
- FR: « Je suppose », dit Holmes, « que lorsque M. Windibank est revenu de France, il a été très contrarié que vous soyez allée au bal. »

**257. `h4-248`** · ch4 ¶38.1 · `ok` · status `silver`

- EN: Did he make no attempt to see you?”
- FR: N'a-t-il fait aucune tentative pour vous voir ? »

**258. `h4-249`** · ch4 ¶41.0 · `ok` · status `silver`

- EN: “Oh, yes, Mr. Holmes.
- FR: « Oh, oui, monsieur Holmes.

**259. `h4-250`** · ch4 ¶45.0 · `ok` · status `silver`

- EN: “He slept on the premises.”
- FR: « Il dormait sur place. »

**260. `h4-251`** · ch4 ¶49.1 · `ok` · status `silver`

- EN: He said that if they were sent to the office he would be chaffed by all the other clerks about having letters from a lady, so I offered to typewrite them, like he did his, but he wouldn’t have that, for he said that when I wrote them they seemed to come from me, but when they were typewritten he always felt that the machine had come between us.
- FR: Il disait que si elles étaient envoyées au bureau, tous les autres employés le taquineraient parce qu’il recevait des lettres d’une femme, alors j’ai proposé de les taper à la machine, comme il le faisait pour les siennes, mais il ne voulait pas de cela, car il disait que lorsque je les écrivais, elles semblaient venir de moi, mais que lorsqu’elles étaient dactylographiées, il sentait toujours que la machine s’était interposée entre nous.

**261. `h4-252`** · ch4 ¶51.0 · `ok` · status `silver`

- EN: “He was a very shy man, Mr. Holmes.
- FR: « C’était un homme très timide, monsieur Holmes.

**262. `h4-253`** · ch4 ¶51.5 · `ok` · status `silver`

- EN: He was always well dressed, very neat and plain, but his eyes were weak, just as mine are, and he wore tinted glasses against the glare.”
- FR: Il était toujours bien habillé, très soigné et sobre, mais ses yeux étaient faibles, tout comme les miens, et il portait des verres teintés contre l’éblouissement. »

**263. `h4-254`** · ch4 ¶53.2 · `ok` · status `silver`

- EN: Mother said he was quite right to make me swear, and that it was a sign of his passion.
- FR: Mère a dit qu'il avait tout à fait raison de me faire jurer, et que c'était une marque de sa passion.

**264. `h4-255`** · ch4 ¶54.0 · `ok` · status `silver`

- EN: “It missed him, then?”
- FR: « Elle l'a donc manqué ? »

**265. `h4-256`** · ch4 ¶57.0 · `ok` · status `silver`

- EN: “Yes, sir, but very quietly.
- FR: « Oui, monsieur, mais très discrètement.

**266. `h4-257`** · ch4 ¶57.5 · `ok` · status `silver`

- EN: That was last Friday, Mr. Holmes, and I have never seen or heard anything since then to throw any light upon what became of him.”
- FR: C'était vendredi dernier, monsieur Holmes, et je n'ai rien vu ni entendu depuis lors qui puisse jeter la moindre lumière sur ce qu'il est devenu. »

**267. `h4-258`** · ch4 ¶60.0 · `ok` · status `silver`

- EN: “Most certainly it does.
- FR: « C'est très certainement le cas.

**268. `h4-259`** · ch4 ¶64.1 · `ok` · status `silver`

- EN: How did your mother take the matter?”
- FR: Comment votre mère a-t-elle pris la chose ? »

**269. `h4-262`** · ch4 ¶71.0 · `ok` · status `silver`

- EN: “Then what has happened to him?”
- FR: « Alors que lui est-il arrivé ? »

**270. `h4-263`** · ch4 ¶75.0 · `ok` · status `silver`

- EN: “No. 31 Lyon Place, Camberwell.”
- FR: « N° 31 Lyon Place, Camberwell. »

**271. `h4-264`** · ch4 ¶78.2 · `ok` · status `silver`

- EN: You will leave the papers here, and remember the advice which I have given you.
- FR: Vous allez laisser les papiers ici, et vous rappeler le conseil que je vous ai donné.

**272. `h4-266`** · ch4 ¶82.0 · `ok` · status `silver`

- EN: “Quite an interesting study, that maiden,” he observed.
- FR: « Une étude tout à fait intéressante, cette jeune fille », observa-t-il.

**273. `h4-267`** · ch4 ¶83.0 · `ok` · status `silver`

- EN: “You appeared to read a good deal upon her which was quite invisible to me,” I remarked.
- FR: « Vous avez paru lire beaucoup de choses sur elle qui m'étaient tout à fait invisibles », remarquai-je.

**274. `h4-268`** · ch4 ¶85.0 · `ok` · status `silver`

- EN: “Well, she had a slate-coloured, broad-brimmed straw hat, with a feather of a brickish red.
- FR: « Eh bien, elle avait un chapeau de paille à large bord, couleur d'ardoise, avec une plume d'un rouge brique.

**275. `h4-270`** · ch4 ¶87.3 · `ok` · status `silver`

- EN: Never trust to general impressions, my boy, but concentrate yourself upon details.
- FR: Ne vous fiez jamais aux impressions générales, mon garçon, mais concentrez-vous sur les détails.

**276. `h4-272`** · ch4 ¶89.2 · `ok` · status `silver`

- EN: One was buttoned only in the two lower buttons out of five, and the other at the first, third, and fifth.
- FR: L'une n'était boutonnée qu'aux deux boutons du bas sur cinq, et l'autre aux premier, troisième et cinquième.

**277. `h4-273`** · ch4 ¶91.2 · `ok` · status `silver`

- EN: She had written in a hurry and dipped her pen too deep.
- FR: Elle avait écrit à la hâte et trempé sa plume trop profondément.

**278. `h4-274`** · ch4 ¶92.1 · `ok` · status `silver`

- EN: “Missing,” it said, “on the morning of the fourteenth, a gentleman named Hosmer Angel.
- FR: « Disparu, disait-il, dans la matinée du quatorze, un monsieur nommé Hosmer Angel.

**279. `h4-275`** · ch4 ¶92.6 · `ok` · status `silver`

- EN: Anybody bringing,” &c, &c.
- FR: Toute personne rapportant... », etc., etc.

**280. `h4-276`** · ch4 ¶94.0 · `ok` · status `silver`

- EN: “They are typewritten,” I remarked.
- FR: « Elles sont écrites à la machine », remarquai-je.

**281. `h4-278`** · ch4 ¶99.2 · `ok` · status `silver`

- EN: One is to a firm in the City, the other is to the young lady’s stepfather, Mr. Windibank, asking him whether he could meet us here at six o’clock to-morrow evening.
- FR: L'une s'adresse à une maison de la City, l'autre au beau-père de la jeune fille, M. Windibank, pour lui demander s'il pourrait nous rencontrer ici demain soir à six heures.

**282. `h4-279`** · ch4 ¶101.0 · `ok` · status `silver`

- EN: I left him then, still puffing at his black clay pipe, with the conviction that when I came again on the next evening I would find that he held in his hands all the clues which would lead up to the identity of the disappearing bridegroom of Miss Mary Sutherland.
- FR: Je le laissai alors, tirant toujours sur sa pipe en terre noire, avec la conviction que lorsque je reviendrais le lendemain soir, je trouverais qu'il tenait entre ses mains tous les indices qui conduiraient à l'identité du fiancé disparu de Miss Mary Sutherland.

**283. `h4-280`** · ch4 ¶103.0 · `ok` · status `silver`

- EN: “Well, have you solved it?”
- FR: « Eh bien, l'avez-vous résolu ? »

**284. `h4-281`** · ch4 ¶106.2 · `ok` · status `silver`

- EN: There was never any mystery in the matter, though, as I said yesterday, some of the details are of interest.
- FR: Il n’y a jamais eu le moindre mystère dans cette affaire, bien que, comme je l'ai dit hier, certains détails présentent de l’intérêt.

**285. `h4-282`** · ch4 ¶109.1 · `ok` · status `silver`

- EN: “He has written to me to say that he would be here at six.
- FR: « Il m’a écrit pour dire qu’il serait ici à six heures.

**286. `h4-283`** · ch4 ¶112.1 · `ok` · status `silver`

- EN: I am afraid that I am a little late, but I am not quite my own master, you know.
- FR: Je crains d'être un peu en retard, mais je ne suis pas tout à fait mon propre maître, vous savez.

**287. `h4-284`** · ch4 ¶112.5 · `ok` · status `silver`

- EN: Besides, it is a useless expense, for how could you possibly find this Hosmer Angel?”
- FR: En outre, c'est une dépense inutile, car comment pourriez-vous bien retrouver cet Hosmer Angel ? »

**288. `h4-285`** · ch4 ¶115.1 · `ok` · status `silver`

- EN: Unless they are quite new, no two of them write exactly alike.
- FR: À moins qu'elles ne soient tout à fait neuves, il n'en est pas deux qui écrivent exactement de la même manière.

**289. `h4-286`** · ch4 ¶117.0 · `ok` · status `silver`

- EN: “And now I will show you what is really a very interesting study, Mr. Windibank,” Holmes continued.
- FR: « Et maintenant je vais vous montrer ce qui est vraiment une étude fort intéressante, M. Windibank », continua Holmes.

**290. `h4-287`** · ch4 ¶117.5 · `ok` · status `silver`

- EN: In each case, not only are the ‘e’s’ slurred and the ‘r’s’ tailless, but you will observe, if you care to use my magnifying lens, that the fourteen other characteristics to which I have alluded are there as well.”
- FR: Dans chaque cas, non seulement les « e » sont pâteux et les « r » sans queue, mais vous observerez, si vous voulez bien utiliser ma loupe, que les quatorze autres particularités auxquelles j'ai fait allusion s'y trouvent également. »

**291. `h4-288`** · ch4 ¶119.1 · `ok` · status `silver`

- EN: “I let you know, then, that I have caught him!”
- FR: « Je vous fais savoir, alors, que je l'ai attrapé ! »

**292. `h4-289`** · ch4 ¶121.4 · `ok` · status `silver`

- EN: Sit down and let us talk it over.”
- FR: Asseyez-vous et parlons-en. »

**293. `h4-292`** · ch4 ¶125.6 · `ok` · status `silver`

- EN: But soon he found that that would not answer forever.
- FR: Mais il découvrit bientôt que cela ne marcherait pas pour toujours.

**294. `h4-293`** · ch4 ¶126.0 · `ok` · status `silver`

- EN: “It was only a joke at first,” groaned our visitor.
- FR: « Ce n'était qu'une plaisanterie au début », gémit notre visiteur.

**295. `h4-294`** · ch4 ¶127.4 · `ok` · status `silver`

- EN: There were meetings, and an engagement, which would finally secure the girl’s affections from turning towards anyone else.
- FR: Il y eut des rencontres, et des fiançailles, qui empêcheraient définitivement les sentiments de la jeune fille de se tourner vers quelqu’un d’autre.

**296. `h4-295`** · ch4 ¶127.9 · `ok` · status `silver`

- EN: James Windibank wished Miss Sutherland to be so bound to Hosmer Angel, and so uncertain as to his fate, that for ten years to come, at any rate, she would not listen to another man.
- FR: James Windibank souhaitait que Mlle Sutherland fût si attachée à Hosmer Angel, et si incertaine quant à son sort, que pendant les dix années à venir, en tout cas, elle n’écouterait aucun autre homme.

**297. `h4-296`** · ch4 ¶129.0 · `ok` · status `silver`

- EN: “It may be so, or it may not, Mr. Holmes,” said he, “but if you are so very sharp you ought to be sharp enough to know that it is you who are breaking the law now, and not me. I have done nothing actionable from the first, but as long as you keep that door locked you lay yourself open to an action for assault and illegal constraint.”
- FR: « Il se peut que ce soit ainsi, ou qu’il n’en soit rien, monsieur Holmes, dit-il, mais si vous êtes si perspicace, vous devriez l’être assez pour savoir que c’est vous qui enfreignez la loi maintenant, et non pas moi. Je n’ai rien fait de passible de poursuites depuis le début, mais tant que vous gardez cette porte verrouillée, vous vous exposez à une action pour voies de fait et séquestration illégale. »

**298. `h4-298`** · ch4 ¶133.1 · `ok` · status `silver`

- EN: Then the fact that the two men were never together, but that the one always appeared when the other was away, was suggestive.
- FR: Ensuite, le fait que les deux hommes ne fussent jamais ensemble, mais que l'un apparût toujours lorsque l'autre était absent, était suggestif.

**299. `h4-299`** · ch4 ¶135.0 · `ok` · status `silver`

- EN: “Having once spotted my man, it was easy to get corroboration.
- FR: « Une fois mon homme repéré, il fut facile d'obtenir une confirmation.

**300. `h4-300`** · ch4 ¶135.5 · `ok` · status `silver`

- EN: As I expected, his reply was typewritten and revealed the same trivial but characteristic defects.
- FR: Comme je m'y attendais, sa réponse était dactylographiée et révélait les mêmes défauts insignifiants mais caractéristiques.

