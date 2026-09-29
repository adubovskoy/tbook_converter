# GOLD-lite review — en→ru (Russian)

Set: `bench-quality/gold/en-ru.jsonl` — 300 sentences, *The Adventures of Sherlock Holmes*, chapters 1–4.

Reference translations: **google/gemini-3.7-flash**, no repair pass. Adversarial check: **claude sonnet (via the claude CLI)**, one verdict per sentence.

Checker verdicts: **wrong 8**, **minor 49**, **ok 243**.

## How to use this file

A human reader of Russian works through this file. For every item, either

- leave it alone — the reference is right: set `"status": "gold"` for that `id` in
  `bench-quality/gold/en-ru.jsonl`; or
- correct `ref` in `bench-quality/gold/en-ru.jsonl` to the translation you would publish, and set
  `"status": "gold"` with `"ref_model": "human"`.

Until an item is touched by a human its status stays as the machine checker left
it (`silver` for `ok`, `disputed` for `minor`/`wrong`). **Disputed items come
first** — they are where the checker and the reference disagree and where a
human pass buys the most.

## Items

### Disputed — checker verdict `wrong`

**1. `h4-018`** · ch2 ¶27.0 · `wrong` · status `disputed`

- EN: “Peculiar—that is the very word,” said Holmes.
- RU: «Необычайно — это самое подходящее слово», — сказал Холмс.
- checker: «Необычайно» (extraordinarily) mistranslates «Peculiar» (odd/strange), shifting the sense

**2. `h4-083`** · ch2 ¶182.0 · `wrong` · status `disputed`

- EN: “I will not look.”
- RU: «Я не буду искать».
- checker: «искать» (search) mistranslates 'look' (watch/observe), changing the meaning

**3. `h4-100`** · ch2 ¶210.3 · `wrong` · status `disputed`

- EN: It might be a satisfaction to his Majesty to regain it with his own hands.”
- RU: Его Величеству доставит удовлетворение вернуть её собственными руками».
- checker: «вернуть» reads as giving the photo back, inverting ‘regain’ (get it back for himself)

**4. `h4-133`** · ch3 ¶34.0 · `wrong` · status `disputed`

- EN: “Well, it is just as I have been telling you, Mr. Sherlock Holmes,” said Jabez Wilson, mopping his forehead; “I have a small pawnbroker’s business at Coburg Square, near the City.
- RU: «Ну, все в точности так, как я вам и говорил, мистер Шерлок Холмс, — сказал Джейбез Уилсон, вытирая лоб, — у меня небольшое дело ростовщика на Кобург-сквер, недалеко от Сити.
- checker: «ростовщик» (usurer/moneylender) misrenders «pawnbroker» (пункт залога), a different business

**5. `h4-211`** · ch3 ¶193.3 · `wrong` · status `disputed`

- EN: Jump, Archie, jump, and I’ll swing for it!”
- RU: Прыгай, Арчи, прыгай, а я возьму вину на себя!
- checker: «swing for it» (be hanged) rendered as «возьму вину на себя» (take the blame), losing the fatal stakes

**6. `h4-230`** · ch4 ¶4.8 · `wrong` · status `disputed`

- EN: The crudest of writers could invent nothing more crude.”
- RU: Самый бездарный из писателей не смог бы придумать ничего более банального.
- checker: the repeated «crude...crude» is split into two unrelated words «бездарный» (untalented) and «банального» (banal), losing the rhetorical echo and the actual meaning of 'crude'

**7. `h4-232`** · ch4 ¶9.0 · `wrong` · status `disputed`

- EN: “It was from the reigning family of Holland, though the matter in which I served them was of such delicacy that I cannot confide it even to you, who have been good enough to chronicle one or two of my little problems.”
- RU: «Оно от правящего дома Голландии, хотя дело, в котором я оказал им услугу, было столь деликатным, что я не могу доверить его даже вам, столь любезно описавшему одну или две из моих маленьких задач».
- checker: «Оно» is neuter, but the antecedent «табакерка» (snuffbox) is feminine — should be «Она»

**8. `h4-259`** · ch4 ¶64.1 · `wrong` · status `disputed`

- EN: How did your mother take the matter?”
- RU: «Ещё один вопрос.
- checker: «Ещё один вопрос» invents unrelated content, dropping the actual question «How did your mother take the matter?»

### Disputed — checker verdict `minor`

**9. `h4-003`** · ch2 ¶1.6 · `minor` · status `disputed`

- EN: He never spoke of the softer passions, save with a gibe and a sneer.
- RU: Он никогда не говорил о более нежных страстях иначе как с насмешкой и презрительной ухмылкой.
- checker: «более нежных страстях» adds an unwarranted comparative to «the softer passions», which is an absolute euphemism for romantic feeling

**10. `h4-010`** · ch2 ¶10.2 · `minor` · status `disputed`

- EN: It is true that I had a country walk on Thursday and came home in a dreadful mess, but as I have changed my clothes I can’t imagine how you deduce it.
- RU: Это правда, что в четверг я совершил загородную прогулку и вернулся домой ужасно перепачканным, но поскольку я переоделся, то ума не приложу, как вы это вывели с помощью дедукции.
- checker: «вывели с помощью дедукции» is redundant padding for the simple «deduce it»

**11. `h4-017`** · ch2 ¶24.4 · `minor` · status `disputed`

- EN: What do you deduce from it?”
- RU: Что вы выводите из неё методом дедукции?»
- checker: adds unsourced «методом дедукции», redundant gloss not in English

**12. `h4-026`** · ch2 ¶40.0 · `minor` · status `disputed`

- EN: A slow and heavy step, which had been heard upon the stairs and in the passage, paused immediately outside the door.
- RU: Медленный и тяжелый шаг, который был слышен на лестнице и в коридоре, затих прямо за дверью.
- checker: «затих» (died away) shifts «paused» from the footstep motion stopping to the sound merely fading

**13. `h4-031`** · ch2 ¶50.0 · `minor` · status `disputed`

- EN: “You will excuse this mask,” continued our strange visitor.
- RU: — Вы извините эту маску, — продолжил наш странный посетитель.
- checker: literal «Вы извините эту маску» treats the mask as the thing forgiven, not the wearing of it — awkward collocation

**14. `h4-038`** · ch2 ¶68.1 · `minor` · status `disputed`

- EN: If this young person should produce her letters for blackmailing or other purposes, how is she to prove their authenticity?”
- RU: Если эта девица предъявит свои письма для шантажа или других целей, как она докажет их подлинность?
- checker: «эта девица» adds a dismissive/pejorative tone absent from neutral «this young person»

**15. `h4-041`** · ch2 ¶87.2 · `minor` · status `disputed`

- EN: Once we diverted her luggage when she travelled.
- RU: Один раз мы перехватили ее багаж во время ее путешествия.
- checker: double «ее... ее» is clunky, and «перехватили» (intercepted) shifts nuance from the milder «diverted»

**16. `h4-057`** · ch2 ¶131.6 · `minor` · status `disputed`

- EN: He is dark, handsome, and dashing, never calls less than once a day, and often twice.
- RU: Он темноволос, красив и лих, заходит не реже одного раза в день, а часто и дважды.
- checker: «лих» is an archaic/unusual short-form adjective for «dashing», slightly odd register

**17. `h4-062`** · ch2 ¶136.1 · `minor` · status `disputed`

- EN: It hadn’t pulled up before she shot out of the hall door and into it.
- RU: Оно ещё не остановилось, как она выскочила из дверей дома и юркнула внутрь.
- checker: «юркнула внутрь» has no clear referent right after «дверей дома», reader may misread it as back into the house rather than into the cab

**18. `h4-063`** · ch2 ¶138.2 · `minor` · status `disputed`

- EN: The driver looked twice at such a shabby fare, but I jumped in before he could object.
- RU: Кучер с сомнением оглядел столь оборванного седока, но я вскочил внутрь прежде, чем он успел возразить.
- checker: «looked twice» (a startled double-take) flattened to «с сомнением оглядел» (looked doubtfully), losing the specific gesture

**19. `h4-066`** · ch2 ¶143.2 · `minor` · status `disputed`

- EN: It was the most preposterous position in which I ever found myself in my life, and it was the thought of it that started me laughing just now.
- RU: Это было самое нелепое положение, в каком я когда-либо оказывался в своей жизни, и именно воспоминание о нем только что рассмешило меня.
- checker: «the thought of it» rendered as «воспоминание о нем» (the memory of it), a slight nuance shift from thought to memory

**20. `h4-073`** · ch2 ¶161.2 · `minor` · status `disputed`

- EN: Do not join in it.
- RU: Не вмешивайтесь в неё.
- checker: «вмешивайтесь» adds a negative/meddling connotation not present in the neutral «join in»

**21. `h4-080`** · ch2 ¶176.3 · `minor` · status `disputed`

- EN: She knows that the King is capable of having her waylaid and searched.
- RU: Она знает, что король способен подстеречь её и обыскать.
- checker: «способен подстеречь её и обыскать» drops the causative nuance of «having her waylaid», implying the King acts in person rather than through agents

**22. `h4-082`** · ch2 ¶178.7 · `minor` · status `disputed`

- EN: It must be where she can lay her hands upon it.
- RU: Она должна быть там, где она сможет наложить на неё руки.
- checker: «наложить на неё руки» is an odd/heavy idiom for a neutral 'get hold of it', echoing the suicide/seizure sense of «наложить руки»

**23. `h4-090`** · ch2 ¶195.1 · `minor` · status `disputed`

- EN: A maid rushed across and threw open the window.
- RU: Горничная бросилась вперед и распахнула окно.
- checker: «бросилась вперед» (rushed forward) loses the nuance of 'across' (the room)

**24. `h4-091`** · ch2 ¶195.6 · `minor` · status `disputed`

- EN: Slipping through the shouting crowd I made my way to the corner of the street, and in ten minutes was rejoiced to find my friend’s arm in mine, and to get away from the scene of uproar.
- RU: Проскользнув сквозь кричащую толпу, я пробрался к углу улицы и через десять минут был счастлив почувствовать руку моего друга под моей рукой и уйти с места переполоха.
- checker: «под моей рукой» is an awkward rendering of arms linked together (‘arm in mine’)

**25. `h4-107`** · ch2 ¶239.0 · `minor` · status `disputed`

- EN: “I am Mr. Holmes,” answered my companion, looking at her with a questioning and rather startled gaze.
- RU: «Я мистер Холмс», — ответил мой спутник, глядя на неё вопросительным и довольно поражённым взглядом.
- checker: «поражённым» overtranslates «startled» as a stronger astonishment

**26. `h4-110`** · ch2 ¶245.1 · `minor` · status `disputed`

- EN: You took me in completely.
- RU: Вы провели меня совершенно.
- checker: «провели меня совершенно» has an unnatural calque-like word order

**27. `h4-113`** · ch2 ¶245.16 · `minor` · status `disputed`

- EN: I love and am loved by a better man than he.
- RU: Я люблю и любима человеком лучшим, чем он.
- checker: awkward word order/missing comma in «человеком лучшим, чем он»

**28. `h4-119`** · ch2 ¶258.2 · `minor` · status `disputed`

- EN: And when he speaks of Irene Adler, or when he refers to her photograph, it is always under the honourable title of the woman.
- RU: И когда он говорит об Ирэн Адлер или когда упоминает её фотографию, это всегда под почётным званием «эта женщина».
- checker: calque-y «это всегда под почётным званием» with unclear referent for «это»

**29. `h4-125`** · ch3 ¶15.0 · `minor` · status `disputed`

- EN: Sherlock Holmes’ quick eye took in my occupation, and he shook his head with a smile as he noticed my questioning glances.
- RU: Быстрый глаз Шерлока Холмса подметил мое занятие, и он с улыбкой покачал головой, заметив мои вопросительные взгляды.
- checker: «быстрый глаз» is a stiff calque of "quick eye", not idiomatic Russian collocation

**30. `h4-127`** · ch3 ¶20.0 · `minor` · status `disputed`

- EN: “I won’t insult your intelligence by telling you how I read that, especially as, rather against the strict rules of your order, you use an arc-and-compass breastpin.”
- RU: — Я не стану оскорблять ваш интеллект, рассказывая вам, как я это понял, тем более что, вопреки строгим правилам вашего ордена, вы носите булавку для галстука с изображением дуги и циркуля».
- checker: «булавка для галстука» narrows "breastpin" to a necktie pin specifically, not stated in source

**31. `h4-132`** · ch3 ¶31.1 · `minor` · status `disputed`

- EN: “It is a little off the beaten track, isn’t it?” said he.
- RU: «Это несколько необычно, не правда ли?» — сказал он.
- checker: «несколько необычно» flattens the spatial idiom «off the beaten track» into vague oddness

**32. `h4-139`** · ch3 ¶46.1 · `minor` · status `disputed`

- EN: ’ he asked with his eyes open.
- RU: — спросил он с широко открытыми глазами.
- checker: «широко открытыми» adds intensity not in plain «eyes open»

**33. `h4-143`** · ch3 ¶57.0 · `minor` · status `disputed`

- EN: “I never hope to see such a sight as that again, Mr. Holmes.
- RU: Я никогда больше не надеюсь увидеть подобное зрелище, мистер Холмс.
- checker: «никогда больше не надеюсь увидеть» is an awkward calque of the English construction, unnatural in Russian

**34. `h4-147`** · ch3 ¶61.4 · `minor` · status `disputed`

- EN: Then suddenly he plunged forward, wrung my hand, and congratulated me warmly on my success.
- RU: Затем внезапно он бросился вперёд, пожал мне руку и горячо поздравил меня с успехом.
- checker: «wrung my hand» flattened to plain «пожал мне руку», losing the vigorous gesture

**35. `h4-167`** · ch3 ¶106.4 · `minor` · status `disputed`

- EN: I did not wish to lose such a place without a struggle, so, as I had heard that you were good enough to give advice to poor folk who were in need of it, I came right away to you.”
- RU: Я не хотел потерять такое место без борьбы, поэтому, так как я слышал, что вы столь любезны, чтобы давать советы бедным людям, которые в них нуждаются, я сразу же пришел к вам».
- checker: awkward calque «столь любезны, чтобы давать» — should be «так любезны, что дают»

**36. `h4-179`** · ch3 ¶138.1 · `minor` · status `disputed`

- EN: It was a poky, little, shabby-genteel place, where four lines of dingy two-storied brick houses looked out into a small railed-in enclosure, where a lawn of weedy grass and a few clumps of faded laurel bushes made a hard fight against a smoke-laden and uncongenial atmosphere.
- RU: Это было убогое, маленькое, претенциозно-бедное местечко, где четыре ряда унылых двухэтажных кирпичных домов выходили на небольшой огороженный перилами скверик, где газон из сорной травы и несколько кустиков увядшего лавра вели упорную борьбу с пропитанной дымом и неблагоприятной атмосферой.
- checker: «претенциозно-бедное» is a debatable stretch for «shabby-genteel» (implies pretension the original doesn't state)

**37. `h4-183`** · ch3 ¶149.2 · `minor` · status `disputed`

- EN: We know something of Saxe-Coburg Square.
- RU: Мы кое-что узнали о Сакс-Кобург-сквер.
- checker: «узнали» shifts present-tense «know» into a completed past discovery

**38. `h4-186`** · ch3 ¶152.3 · `minor` · status `disputed`

- EN: The swing of his nature took him from extreme languor to devouring energy; and, as I knew well, he was never so truly formidable as when, for days on end, he had been lounging in his armchair amid his improvisations and his black-letter editions.
- RU: Колебания его натуры бросали его от крайнего бессилия к пожирающей энергии; и, как я хорошо знал, он никогда не был настолько по-настоящему грозен, как в те моменты, когда целыми днями напролет бездельничал в своем кресле среди своих импровизаций и своих старинных готических изданий.
- checker: «бессилие» (powerlessness) for «languor» loses the listless/lethargic nuance

**39. `h4-193`** · ch3 ¶167.2 · `minor` · status `disputed`

- EN: It is not too much to say that once or twice, as in that business of the Sholto murder and the Agra treasure, he has been more nearly correct than the official force.”
- RU: Не будет преувеличением сказать, что раз или два, как в том деле с убийством Шолто и сокровищами Агры, он был ближе к истине, чем официальные силы».
- checker: «официальные силы» is vague for «the official force» (police), context needed to disambiguate

**40. `h4-195`** · ch3 ¶170.3 · `minor` · status `disputed`

- EN: His grandfather was a royal duke, and he himself has been to Eton and Oxford.
- RU: Его дед был королевским герцогом, а сам он учился в Итоне и Оксфорде.
- checker: «королевским герцогом» is an odd calque for the peerage title «royal duke» (a duke of royal blood)

**41. `h4-199`** · ch3 ¶174.3 · `minor` · status `disputed`

- EN: This also was opened, and led down a flight of winding stone steps, which terminated at another formidable gate.
- RU: Ее тоже открыли, и она вела вниз по винтовой каменной лестнице, которая упиралась в еще одну внушительную дверь.
- checker: «дверь» flattens «formidable gate» into a plain door, losing the heavier barrier image

**42. `h4-201`** · ch3 ¶178.1 · `minor` · status `disputed`

- EN: A few seconds sufficed to satisfy him, for he sprang to his feet again and put his glass in his pocket.
- RU: Нескольких секунд оказалось достаточно, чтобы удовлетворить его, ибо он снова вскочил на ноги и убрал лупу в карман.
- checker: «удовлетворить его» is an odd collocation for satisfying one's own inspection/curiosity, not a person's demand

**43. `h4-212`** · ch3 ¶195.0 · `minor` · status `disputed`

- EN: “It’s no use, John Clay,” said Holmes blandly.
- RU: — Бесполезно, Джон Клей, — мягко сказал Холмс.
- checker: «blandly» (flat, dry tone) mistranslated as «мягко» (gently/softly)

**44. `h4-214`** · ch3 ¶200.1 · `minor` · status `disputed`

- EN: “He’s quicker at climbing down holes than I am.
- RU: — Он ныряет в дыры проворнее меня.
- checker: «climbing down holes» rendered as «ныряет в дыры» (dives into), shifting the verb's meaning

**45. `h4-216`** · ch3 ¶204.1 · `minor` · status `disputed`

- EN: There is no doubt that you have detected and defeated in the most complete manner one of the most determined attempts at bank robbery that have ever come within my experience.”
- RU: Нет сомнений, что вы самым совершенным образом раскрыли и предотвратили одну из самых дерзких попыток ограбления банка, с какими мне когда-либо приходилось сталкиваться за всю мою практику.
- checker: «most determined attempts» rendered as «самых дерзких попыток» (most daring), shifting resolve to boldness

**46. `h4-225`** · ch3 ¶213.3 · `minor` · status `disputed`

- EN: My life is spent in one long effort to escape from the commonplaces of existence.
- RU: Моя жизнь проходит в одном долгом усилии убежать от банальностей существования.
- checker: «в одном долгом усилии» is a stiff calque of the English syntax

**47. `h4-237`** · ch4 ¶16.1 · `minor` · status `disputed`

- EN: Then, suddenly realising the full purport of his words, she gave a violent start and looked up, with fear and astonishment upon her broad, good-humoured face.
- RU: Затем, внезапно осознав полный смысл его слов, она сильно вздрогнула и подняла взгляд, со страхом и изумлением на своем широком, добродушном лице.
- checker: awkward comma splice: «подняла взгляд, со страхом и изумлением на... лице» reads clumsily

**48. `h4-239`** · ch4 ¶20.1 · `minor` · status `disputed`

- EN: “Yes, I did bang out of the house,” she said, “for it made me angry to see the easy way in which Mr. Windibank—that is, my father—took it all.
- RU: — Да, я действительно выскочила из дома, хлопнув дверью, — сказала она, — потому что меня разозлило то, с каким легкомыслием мистер Виндибэнк — то есть мой отец — воспринял всё это.
- checker: «хлопнув дверью» adds a door-slamming detail not present in the source

**49. `h4-250`** · ch4 ¶45.0 · `minor` · status `disputed`

- EN: “He slept on the premises.”
- RU: — Он ночевал в конторе.
- checker: «в конторе» narrows generic «premises» to a specific office without textual basis

**50. `h4-253`** · ch4 ¶51.5 · `minor` · status `disputed`

- EN: He was always well dressed, very neat and plain, but his eyes were weak, just as mine are, and he wore tinted glasses against the glare.”
- RU: Он всегда был хорошо одет, очень опрятно и просто, но его глаза были слабыми, совсем как мои, и он носил тонированные очки от яркого света.
- checker: «тонированные очки» is a modern/automotive register, anachronistic for Victorian «tinted glasses»

**51. `h4-260`** · ch4 ¶67.1 · `minor` · status `disputed`

- EN: As he said, what interest could anyone have in bringing me to the doors of the church, and then leaving me?
- RU: Как он сказал, какой интерес мог быть у кого-либо довести меня до дверей церкви, а затем бросить?
- checker: «какой интерес мог быть у кого-либо довести меня» is ungrammatical, missing «в том, чтобы» before the infinitive

**52. `h4-269`** · ch4 ¶85.5 · `minor` · status `disputed`

- EN: She had small round, hanging gold earrings, and a general air of being fairly well-to-do in a vulgar, comfortable, easy-going way.”
- RU: У неё были маленькие круглые висячие золотые серьги и общий вид человека довольно зажиточного в вульгарной, уютной, беспечной манере».
- checker: «уютной» renders material «comfortable» (well-to-do) as physical coziness, an odd fit for «манере»

**53. `h4-273`** · ch4 ¶91.2 · `minor` · status `disputed`

- EN: She had written in a hurry and dipped her pen too deep.
- RU: Она писала в спешке и макнула перо слишком глубоко.
- checker: «макнула» is a colloquial verb for dipping a pen, register slip from literary «обмакнула»

**54. `h4-279`** · ch4 ¶101.0 · `minor` · status `disputed`

- EN: I left him then, still puffing at his black clay pipe, with the conviction that when I came again on the next evening I would find that he held in his hands all the clues which would lead up to the identity of the disappearing bridegroom of Miss Mary Sutherland.
- RU: Я оставил его тогда, всё ещё пыхтящего своей чёрной глиняной трубкой, с убеждением, что, когда я приду снова следующим вечером, я обнаружу, что он держит в руках все нити, которые приведут к установлению личности исчезающего жениха мисс Мэри Сазерленд.
- checker: «исчезающего жениха» uses imperfective (ongoing) aspect for a man who already vanished; should be «исчезнувшего»

**55. `h4-291`** · ch4 ¶125.1 · `minor` · status `disputed`

- EN: It was a considerable sum, for people in their position, and the loss of it would have made a serious difference.
- RU: Это была значительная сумма для людей в их положении, и потеря ее составила бы серьезную разницу.
- checker: «составила бы серьезную разницу» is an awkward calque of «made a serious difference», not natural Russian phrasing

**56. `h4-294`** · ch4 ¶127.4 · `minor` · status `disputed`

- EN: There were meetings, and an engagement, which would finally secure the girl’s affections from turning towards anyone else.
- RU: Были встречи и помолвка, которая окончательно оградила бы чувства девушки от обращения к кому-либо другому.
- checker: «оградила бы чувства девушки от обращения к кому-либо другому» is an unnatural calque, hard to parse

**57. `h4-299`** · ch4 ¶135.0 · `minor` · status `disputed`

- EN: “Having once spotted my man, it was easy to get corroboration.
- RU: «Один раз определив своего человека, было легко получить подтверждение.
- checker: «Один раз определив своего человека» ambiguously reads as counting instances rather than «once/as soon as I spotted him»

### Checker verdict `ok` (status `silver`)

**58. `h4-001`** · ch1 ¶0.0 · `ok` · status `silver`

- EN: The Adventures of Sherlock Holmes
- RU: Приключения Шерлока Холмса

**59. `h4-002`** · ch2 ¶1.1 · `ok` · status `silver`

- EN: I have seldom heard him mention her under any other name.
- RU: Я редко слышал, чтобы он упоминал ее под каким-либо другим именем.

**60. `h4-004`** · ch2 ¶2.0 · `ok` · status `silver`

- EN: I had seen little of Holmes lately.
- RU: В последнее время я редко виделся с Холмсом.

**61. `h4-005`** · ch2 ¶2.5 · `ok` · status `silver`

- EN: Beyond these signs of his activity, however, which I merely shared with all the readers of the daily press, I knew little of my former friend and companion.
- RU: Однако, помимо этих свидетельств его деятельности, о которых я узнавал наряду со всеми читателями ежедневной прессы, я мало что знал о моем прежнем друге и компаньоне.

**62. `h4-006`** · ch2 ¶3.4 · `ok` · status `silver`

- EN: To me, who knew his every mood and habit, his attitude and manner told their own story.
- RU: Мне, знавшему каждое его настроение и привычку, его поза и манеры рассказывали свою собственную историю.

**63. `h4-007`** · ch2 ¶4.1 · `ok` · status `silver`

- EN: It seldom was; but he was glad, I think, to see me.
- RU: Она редко бывала таковой; но он был рад, я думаю, видеть меня.

**64. `h4-008`** · ch2 ¶5.1 · `ok` · status `silver`

- EN: “I think, Watson, that you have put on seven and a half pounds since I saw you.”
- RU: «Я думаю, Ватсон, что вы прибавили семь с половиной фунтов с тех пор, как я вас видел».

**65. `h4-009`** · ch2 ¶8.0 · `ok` · status `silver`

- EN: “Then, how do you know?”
- RU: «Тогда как же вы это узнали?»

**66. `h4-011`** · ch2 ¶12.2 · `ok` · status `silver`

- EN: Hence, you see, my double deduction that you had been out in vile weather, and that you had a particularly malignant boot-slitting specimen of the London slavey.
- RU: Отсюда, как видите, моя двойная дедукция о том, что вы выходили в мерзкую погоду и что у вас водится особо зловредный, режущий обувь образец лондонской служанки.

**67. `h4-012`** · ch2 ¶14.0 · `ok` · status `silver`

- EN: “Quite so,” he answered, lighting a cigarette, and throwing himself down into an armchair.
- RU: «Совершенно верно, — ответил он, закуривая сигарету и опускаясь в кресло. —

**68. `h4-013`** · ch2 ¶18.0 · `ok` · status `silver`

- EN: “Then how many are there?”
- RU: «Тогда сколько их там?»

**69. `h4-014`** · ch2 ¶20.4 · `ok` · status `silver`

- EN: Now, I know that there are seventeen steps, because I have both seen and observed.
- RU: А я вот знаю, что там семнадцать ступеней, потому что я и видел, и наблюдал.

**70. `h4-015`** · ch2 ¶22.0 · `ok` · status `silver`

- EN: “There will call upon you to-night, at a quarter to eight o’clock,” it said, “a gentleman who desires to consult you upon a matter of the very deepest moment.
- RU: «К вам явится сегодня вечером, без четверти восемь, — гласила она, — джентльмен, желающий проконсультироваться с вами по делу величайшей важности.

**71. `h4-016`** · ch2 ¶23.1 · `ok` · status `silver`

- EN: “What do you imagine that it means?”
- RU: «Как вы полагаете, что это значит?»

**72. `h4-019`** · ch2 ¶30.0 · `ok` · status `silver`

- EN: “The name of the maker, no doubt; or his monogram, rather.”
- RU: «Имя изготовителя, без сомнения; или, скорее, его вензель».

**73. `h4-020`** · ch2 ¶31.4 · `ok` · status `silver`

- EN: Now for the ‘Eg.’
- RU: Теперь о „Eg“.

**74. `h4-021`** · ch2 ¶31.9 · `ok` · status `silver`

- EN: ‘Remarkable as being the scene of the death of Wallenstein, and for its numerous glass-factories and paper-mills.’
- RU: «Примечательна тем, что является местом смерти Валленштейна, а также своими многочисленными стекольными заводами и бумажными фабриками».

**75. `h4-022`** · ch2 ¶33.2 · `ok` · status `silver`

- EN: Do you note the peculiar construction of the sentence—‘This account of you we have from all quarters received.
- RU: Заметили ли вы своеобразное построение предложения — «Сей отзыв о вас мы со всех сторон получили».

**76. `h4-023`** · ch2 ¶34.0 · `ok` · status `silver`

- EN: As he spoke there was the sharp sound of horses’ hoofs and grating wheels against the curb, followed by a sharp pull at the bell.
- RU: Пока он говорил, раздался отчетливый стук лошадиных копыт и скрежет колес о бордюр, за которыми последовал резкий звонок в дверь.

**77. `h4-024`** · ch2 ¶35.4 · `ok` · status `silver`

- EN: There’s money in this case, Watson, if there is nothing else.”
- RU: В этом деле есть деньги, Ватсон, если даже нет ничего другого».

**78. `h4-025`** · ch2 ¶37.3 · `ok` · status `silver`

- EN: And this promises to be interesting.
- RU: И это обещает быть интересным.

**79. `h4-027`** · ch2 ¶42.2 · `ok` · status `silver`

- EN: Heavy bands of astrakhan were slashed across the sleeves and fronts of his double-breasted coat, while the deep blue cloak which was thrown over his shoulders was lined with flame-coloured silk and secured at the neck with a brooch which consisted of a single flaming beryl.
- RU: Тяжелые полосы каракуля были нашиты поперек рукавов и бортов его двубортного пальто, в то время как темно-синий плащ, наброшенный на его плечи, был подбит шелком цвета пламени и скреплен у шеи брошью, состоявшей из одного пылающего берилла.

**80. `h4-028`** · ch2 ¶43.1 · `ok` · status `silver`

- EN: “I told you that I would call.”
- RU: — Я говорил вам, что зайду.

**81. `h4-029`** · ch2 ¶45.0 · `ok` · status `silver`

- EN: “You may address me as the Count Von Kramm, a Bohemian nobleman.
- RU: — Вы можете называть меня графом фон Краммом, дворянином из Богемии.

**82. `h4-030`** · ch2 ¶46.2 · `ok` · status `silver`

- EN: “You may say before this gentleman anything which you may say to me.”
- RU: — Вы можете сказать в присутствии этого джентльмена всё то, что можете сказать мне.

**83. `h4-032`** · ch2 ¶52.1 · `ok` · status `silver`

- EN: To speak plainly, the matter implicates the great House of Ormstein, hereditary kings of Bohemia.”
- RU: Говоря прямо, дело затрагивает великий дом Ормштейнов, потомственных королей Богемии.

**84. `h4-033`** · ch2 ¶56.0 · `ok` · status `silver`

- EN: The man sprang from his chair and paced up and down the room in uncontrollable agitation.
- RU: Мужчина вскочил со стула и зашагал взад и вперед по комнате в непреодолимом волнении.

**85. `h4-034`** · ch2 ¶57.1 · `ok` · status `silver`

- EN: “Your Majesty had not spoken before I was aware that I was addressing Wilhelm Gottsreich Sigismond von Ormstein, Grand Duke of Cassel-Felstein, and hereditary King of Bohemia.”
- RU: — Ваше Величество еще не заговорили, как я уже знал, что обращаюсь к Вильгельму Готтсрейху Сигизмунду фон Ормштейну, великому герцогу Кассель-Фельштейнскому и потомственному королю Богемии.

**86. `h4-035`** · ch2 ¶60.0 · `ok` · status `silver`

- EN: “The facts are briefly these: Some five years ago, during a lengthy visit to Warsaw, I made the acquaintance of the well-known adventuress, Irene Adler.
- RU: — Факты кратко таковы: около пяти лет назад, во время длительного визита в Варшаву, я познакомился с известной авантюристкой, Ирэн Адлер.

**87. `h4-036`** · ch2 ¶62.0 · `ok` · status `silver`

- EN: “Let me see!” said Holmes.
- RU: — Посмотрим! — сказал Холмс.

**88. `h4-037`** · ch2 ¶62.8 · `ok` · status `silver`

- EN: Your Majesty, as I understand, became entangled with this young person, wrote her some compromising letters, and is now desirous of getting those letters back.”
- RU: Ваше Величество, насколько я понимаю, связались с этой молодой особой, написали ей несколько компрометирующих писем и теперь желаете вернуть эти письма обратно.

**89. `h4-039`** · ch2 ¶80.0 · `ok` · status `silver`

- EN: “You have compromised yourself seriously.”
- RU: — Вы серьезно скомпрометировали себя.

**90. `h4-040`** · ch2 ¶84.0 · `ok` · status `silver`

- EN: “Your Majesty must pay.
- RU: — Ваше Величество должны заплатить.

**91. `h4-042`** · ch2 ¶91.0 · `ok` · status `silver`

- EN: “But a very serious one to me,” returned the King reproachfully.
- RU: — Но очень серьезная для меня, — с укоризной возразил король.

**92. `h4-043`** · ch2 ¶97.1 · `ok` · status `silver`

- EN: You may know the strict principles of her family.
- RU: Вам, возможно, известны строгие принципы ее семьи.

**93. `h4-044`** · ch2 ¶99.1 · `ok` · status `silver`

- EN: And she will do it.
- RU: И она сделает это.

**94. `h4-045`** · ch2 ¶100.0 · `ok` · status `silver`

- EN: “You are sure that she has not sent it yet?”
- RU: — Вы уверены, что она ещё не отправила её?

**95. `h4-046`** · ch2 ¶104.2 · `ok` · status `silver`

- EN: Your Majesty will, of course, stay in London for the present?”
- RU: Ваше Величество, конечно же, пока останется в Лондоне?

**96. `h4-047`** · ch2 ¶109.0 · `ok` · status `silver`

- EN: “You have carte blanche.”
- RU: — У вас карт-бланш.

**97. `h4-048`** · ch2 ¶115.0 · `ok` · status `silver`

- EN: Holmes scribbled a receipt upon a sheet of his note-book and handed it to him.
- RU: Холмс быстро набросал расписку на листке из своей записной книжки и передал ее ему.

**98. `h4-049`** · ch2 ¶118.2 · `ok` · status `silver`

- EN: “Was the photograph a cabinet?”
- RU: — Была ли фотография кабинетного формата?

**99. `h4-050`** · ch2 ¶122.0 · `ok` · status `silver`

- EN: At three o’clock precisely I was at Baker Street, but Holmes had not yet returned.
- RU: Ровно в три часа я был на Бейкер-стрит, но Холмс ещё не вернулся.

**100. `h4-051`** · ch2 ¶122.5 · `ok` · status `silver`

- EN: So accustomed was I to his invariable success that the very possibility of his failing had ceased to enter into my head.
- RU: Я настолько привык к его неизменному успеху, что сама возможность его неудачи перестала приходить мне в голову.

**101. `h4-052`** · ch2 ¶124.0 · `ok` · status `silver`

- EN: “Well, really!” he cried, and then he choked and laughed again until he was obliged to lie back, limp and helpless, in the chair.
- RU: — Ну, право же! — воскликнул он, а затем задохнулся от смеха и снова захохотал, пока не был вынужден откинуться назад, обмякший и беспомощный, в кресле.

**102. `h4-053`** · ch2 ¶128.1 · `ok` · status `silver`

- EN: I will tell you, however.
- RU: Впрочем, я вам расскажу.

**103. `h4-054`** · ch2 ¶128.6 · `ok` · status `silver`

- EN: It is a bijou villa, with a garden at the back, but built out in front right up to the road, two stories.
- RU: Это прелестная миниатюрная вилла с садом позади, но выходящая фасадом прямо к дороге, в два этажа.

**104. `h4-055`** · ch2 ¶129.0 · `ok` · status `silver`

- EN: “I then lounged down the street and found, as I expected, that there was a mews in a lane which runs down by one wall of the garden.
- RU: Затем я не спеша пошел вниз по улице и обнаружил, как и ожидал, что в переулке, идущем вдоль одной из стен сада, находятся конюшни.

**105. `h4-056`** · ch2 ¶131.1 · `ok` · status `silver`

- EN: She is the daintiest thing under a bonnet on this planet.
- RU: Она — самое прелестное создание в шляпке на этой планете.

**106. `h4-058`** · ch2 ¶132.0 · `ok` · status `silver`

- EN: “This Godfrey Norton was evidently an important factor in the matter.
- RU: Этот Годфри Нортон был, очевидно, важным фактором в этом деле.

**107. `h4-059`** · ch2 ¶132.6 · `ok` · status `silver`

- EN: If the latter, it was less likely.
- RU: Если последнее, то это было менее вероятно.

**108. `h4-060`** · ch2 ¶134.0 · `ok` · status `silver`

- EN: “I was still balancing the matter in my mind when a hansom cab drove up to Briony Lodge, and a gentleman sprang out.
- RU: «Я всё ещё взвешивал этот вопрос в уме, когда к Бриони-Лодж подъехал двухколёсный кэб, и из него выскочил джентльмен.

**109. `h4-061`** · ch2 ¶135.2 · `ok` · status `silver`

- EN: Presently he emerged, looking even more flurried than before.
- RU: Вскоре он вышел, выглядя ещё более взволнованным, чем прежде.

**110. `h4-064`** · ch2 ¶139.2 · `ok` · status `silver`

- EN: The cab and the landau with their steaming horses were in front of the door when I arrived.
- RU: Кэб и ландо с их взмыленными лошадьми стояли перед дверями, когда я прибыл.

**111. `h4-065`** · ch2 ¶139.7 · `ok` · status `silver`

- EN: Suddenly, to my surprise, the three at the altar faced round to me, and Godfrey Norton came running as hard as he could towards me.
- RU: Внезапно, к моему удивлению, троица у алтаря повернулась ко мне, и Годфри Нортон со всех ног бросился ко мне навстречу.

**112. `h4-067`** · ch2 ¶145.1 · `ok` · status `silver`

- EN: It looked as if the pair might take an immediate departure, and so necessitate very prompt and energetic measures on my part.
- RU: Похоже было, что эта пара может немедленно уехать, и тем самым вынудить меня к самым быстрым и решительным мерам.

**113. `h4-068`** · ch2 ¶145.5 · `ok` · status `silver`

- EN: They drove away in different directions, and I went off to make my own arrangements.”
- RU: Они разъехались в разные стороны, а я отправился делать свои приготовления».

**114. `h4-069`** · ch2 ¶149.0 · `ok` · status `silver`

- EN: “You don’t mind breaking the law?”
- RU: «Вы не возражаете против нарушения закона?»

**115. `h4-070`** · ch2 ¶154.0 · `ok` · status `silver`

- EN: “Then I am your man.”
- RU: «Тогда я целиком в вашем распоряжении».

**116. `h4-071`** · ch2 ¶157.2 · `ok` · status `silver`

- EN: It is nearly five now.
- RU: Сейчас уже почти пять.

**117. `h4-072`** · ch2 ¶159.1 · `ok` · status `silver`

- EN: There is only one point on which I must insist.
- RU: Есть только один пункт, на котором я должен настоять.

**118. `h4-074`** · ch2 ¶163.0 · `ok` · status `silver`

- EN: “You are to watch me, for I will be visible to you.”
- RU: «Вы должны наблюдать за мной, ибо я буду виден вам».

**119. `h4-075`** · ch2 ¶167.2 · `ok` · status `silver`

- EN: Your task is confined to that.
- RU: Ваша задача ограничивается этим.

**120. `h4-076`** · ch2 ¶170.0 · `ok` · status `silver`

- EN: “Then you may entirely rely on me.”
- RU: — Тогда вы можете полностью на меня положиться.

**121. `h4-077`** · ch2 ¶172.3 · `ok` · status `silver`

- EN: His expression, his manner, his very soul seemed to vary with every fresh part that he assumed.
- RU: Выражение его лица, его манеры, сама его душа, казалось, менялись с каждой новой ролью, которую он принимал.

**122. `h4-078`** · ch2 ¶173.3 · `ok` · status `silver`

- EN: On the contrary, for a small street in a quiet neighbourhood, it was remarkably animated.
- RU: Напротив, для маленькой улицы в тихом районе она была удивительно оживленной.

**123. `h4-079`** · ch2 ¶174.3 · `ok` · status `silver`

- EN: Now the question is, Where are we to find the photograph?”
- RU: Теперь вопрос в том, где нам найти фотографию?»

**124. `h4-081`** · ch2 ¶178.2 · `ok` · status `silver`

- EN: But I am inclined to think neither.
- RU: Но я склонен думать, что ни у того, ни у другого.

**125. `h4-084`** · ch2 ¶186.2 · `ok` · status `silver`

- EN: It is her carriage.
- RU: Это её экипаж.

**126. `h4-085`** · ch2 ¶187.3 · `ok` · status `silver`

- EN: A fierce quarrel broke out, which was increased by the two guardsmen, who took sides with one of the loungers, and by the scissors-grinder, who was equally hot upon the other side.
- RU: Вспыхнула ожесточенная ссора, которую усилили двое гвардейцев, вставших на сторону одного из зевак, и точильщик ножниц, который с таким же пылом принял другую сторону.

**127. `h4-086`** · ch2 ¶187.7 · `ok` · status `silver`

- EN: Irene Adler, as I will still call her, had hurried up the steps; but she stood at the top with her superb figure outlined against the lights of the hall, looking back into the street.
- RU: Ирэн Адлер, как я все еще буду называть ее, поспешила вверх по ступенькам; но она остановилась наверху, и ее великолепная фигура вырисовывалась на фоне огней холла, пока она оглядывалась назад на улицу.

**128. `h4-087`** · ch2 ¶191.0 · `ok` · status `silver`

- EN: “He’s a brave fellow,” said a woman.
- RU: — Он храбрый парень, — сказала какая-то женщина.

**129. `h4-088`** · ch2 ¶192.1 · `ok` · status `silver`

- EN: May we bring him in, marm?”
- RU: Можно нам внести его, мэм?»

**130. `h4-089`** · ch2 ¶194.2 · `ok` · status `silver`

- EN: I do not know whether he was seized with compunction at that moment for the part he was playing, but I know that I never felt more heartily ashamed of myself in my life than when I saw the beautiful creature against whom I was conspiring, or the grace and kindliness with which she waited upon the injured man.
- RU: Я не знаю, охватило ли его в тот момент раскаяние за роль, которую он играл, но знаю, что никогда в жизни не чувствовал более искреннего стыда за себя, чем когда увидел прекрасное создание, против которого я строил козни, и то изящество и доброту, с которыми она ухаживала за раненым.

**131. `h4-092`** · ch2 ¶196.2 · `ok` · status `silver`

- EN: It is all right.”
- RU: Все в полном порядке.

**132. `h4-093`** · ch2 ¶201.0 · `ok` · status `silver`

- EN: “I am still in the dark.”
- RU: — Я все еще в неведении.

**133. `h4-094`** · ch2 ¶203.0 · `ok` · status `silver`

- EN: “I guessed as much.”
- RU: — Я так и догадался.

**134. `h4-095`** · ch2 ¶206.0 · `ok` · status `silver`

- EN: “Then they carried me in.
- RU: — Затем меня внесли внутрь.

**135. `h4-096`** · ch2 ¶206.5 · `ok` · status `silver`

- EN: They laid me on a couch, I motioned for air, they were compelled to open the window, and you had your chance.”
- RU: Они положили меня на кушетку, я показал знаками, что мне нужен воздух, они были вынуждены открыть окно, и вы получили свой шанс».

**136. `h4-097`** · ch2 ¶208.4 · `ok` · status `silver`

- EN: A married woman grabs at her baby; an unmarried one reaches for her jewel-box.
- RU: Замужняя женщина хватает своего ребенка; незамужняя тянется к своей шкатулке с драгоценностями.

**137. `h4-098`** · ch2 ¶208.8 · `ok` · status `silver`

- EN: The smoke and shouting were enough to shake nerves of steel.
- RU: Дыма и криков было достаточно, чтобы потрясти стальные нервы.

**138. `h4-099`** · ch2 ¶208.14 · `ok` · status `silver`

- EN: I hesitated whether to attempt to secure the photograph at once; but the coachman had come in, and as he was watching me narrowly, it seemed safer to wait.
- RU: Я колебался, не попытаться ли завладеть фотографией тотчас же; но вошёл кучер, и поскольку он пристально наблюдал за мной, казалось более безопасным подождать.

**139. `h4-101`** · ch2 ¶212.3 · `ok` · status `silver`

- EN: I must wire to the King without delay.”
- RU: Я должен протелеграфировать королю без промедления.

**140. `h4-102`** · ch2 ¶216.0 · `ok` · status `silver`

- EN: “I’ve heard that voice before,” said Holmes, staring down the dimly lit street.
- RU: — Я уже слышал этот голос раньше, — сказал Холмс, глядя вдоль тускло освещенной улицы.

**141. `h4-103`** · ch2 ¶223.1 · `ok` · status `silver`

- EN: I am all impatience to be gone.”
- RU: Я весь горю нетерпением отправиться.

**142. `h4-104`** · ch2 ¶226.1 · `ok` · status `silver`

- EN: We descended and started off once more for Briony Lodge.
- RU: Мы спустились и снова отправились в Бриони-Лодж.

**143. `h4-105`** · ch2 ¶234.0 · `ok` · status `silver`

- EN: “And why in hopes?”
- RU: — И почему же вы надеетесь?

**144. `h4-106`** · ch2 ¶236.4 · `ok` · status `silver`

- EN: What a queen she would have made!”
- RU: Какой королевой она могла бы стать!»

**145. `h4-108`** · ch2 ¶243.0 · `ok` · status `silver`

- EN: “And the papers?” asked the King hoarsely.
- RU: — А бумаги? — хрипло спросил король.

**146. `h4-109`** · ch2 ¶244.5 · `ok` · status `silver`

- EN: To be left till called for.”
- RU: До востребования».

**147. `h4-111`** · ch2 ¶245.6 · `ok` · status `silver`

- EN: And your address had been given me.
- RU: И мне дали ваш адрес.

**148. `h4-112`** · ch2 ¶245.11 · `ok` · status `silver`

- EN: I sent John, the coachman, to watch you, ran upstairs, got into my walking clothes, as I call them, and came down just as you departed.
- RU: Я послала кучера Джона следить за вами, взбежала наверх, переоделась в свой прогулочный костюм, как я его называю, и спустилась вниз как раз в тот момент, когда вы уходили.

**149. `h4-114`** · ch2 ¶247.0 · `ok` · status `silver`

- EN: “What a woman—oh, what a woman!” cried the King of Bohemia, when we had all three read this epistle.
- RU: — Какая женщина — о, какая женщина! — воскликнул король Богемии, когда мы все трое прочитали это послание.

**150. `h4-115`** · ch2 ¶248.1 · `ok` · status `silver`

- EN: “I am sorry that I have not been able to bring your Majesty’s business to a more successful conclusion.”
- RU: — Мне жаль, что я не смог довести дело вашего Величества до более успешного завершения.

**151. `h4-116`** · ch2 ¶250.0 · `ok` · status `silver`

- EN: “I am glad to hear your Majesty say so.”
- RU: — Я рад слышать, что ваше Величество так говорит.

**152. `h4-117`** · ch2 ¶253.0 · `ok` · status `silver`

- EN: “You have but to name it.”
- RU: — Вам стоит лишь назвать это.

**153. `h4-118`** · ch2 ¶257.1 · `ok` · status `silver`

- EN: Then there is no more to be done in the matter.
- RU: Тогда в этом деле больше нечего делать.

**154. `h4-120`** · ch3 ¶5.0 · `ok` · status `silver`

- EN: “Then I can wait in the next room.”
- RU: — Тогда я могу подождать в соседней комнате.

**155. `h4-121`** · ch3 ¶8.2 · `ok` · status `silver`

- EN: You have shown your relish for it by the enthusiasm which has prompted you to chronicle, and, if you will excuse my saying so, somewhat to embellish so many of my own little adventures.”
- RU: Вы проявили свой вкус к этому тем энтузиазмом, который побудил вас вести летопись и, да простите вы мне эти слова, несколько приукрашивать столь многие из моих собственных скромных приключений.

**156. `h4-122`** · ch3 ¶12.0 · `ok` · status `silver`

- EN: “You did, Doctor, but none the less you must come round to my view, for otherwise I shall keep on piling fact upon fact on you until your reason breaks down under them and acknowledges me to be right.
- RU: — Вы усомнились, доктор, но тем не менее вы должны склониться к моей точке зрения, ибо иначе я буду продолжать нагромождать для вас факт на факт, пока ваш разум не сломится под ними и не признает мою правоту.

**157. `h4-123`** · ch3 ¶12.5 · `ok` · status `silver`

- EN: I ask you not merely because my friend Dr. Watson has not heard the opening part but also because the peculiar nature of the story makes me anxious to have every possible detail from your lips.
- RU: Я прошу вас об этом не только потому, что мой друг доктор Ватсон не слышал начальной части, но и потому, что странный характер этой истории заставляет меня страстно желать услышать из ваших уст каждую возможную подробность.

**158. `h4-124`** · ch3 ¶14.0 · `ok` · status `silver`

- EN: I did not gain very much, however, by my inspection.
- RU: Однако из своего осмотра я извлек не очень много.

**159. `h4-126`** · ch3 ¶17.2 · `ok` · status `silver`

- EN: It’s as true as gospel, for I began as a ship’s carpenter.”
- RU: Это чистая правда, ведь я начинал корабельным плотником».

**160. `h4-128`** · ch3 ¶24.1 · `ok` · status `silver`

- EN: I have made a small study of tattoo marks and have even contributed to the literature of the subject.
- RU: Я провёл небольшое исследование татуировок и даже внёс вклад в литературу по этому предмету.

**161. `h4-129`** · ch3 ¶25.2 · `ok` · status `silver`

- EN: “I thought at first that you had done something clever, but I see that there was nothing in it after all.”
- RU: — Сначала я подумал, что вы совершили нечто умное, но теперь вижу, что в этом вообще ничего не было.

**162. `h4-130`** · ch3 ¶27.2 · `ok` · status `silver`

- EN: This is what began it all.
- RU: Вот с чего всё это началось.

**163. `h4-131`** · ch3 ¶29.2 · `ok` · status `silver`

- EN: All red-headed men who are sound in body and mind and above the age of twenty-one years, are eligible.
- RU: Право на подачу заявления имеют все рыжеволосые мужчины в здравом уме и телесном здоровье, достигшие возраста двадцати одного года.

**164. `h4-134`** · ch3 ¶36.0 · `ok` · status `silver`

- EN: “His name is Vincent Spaulding, and he’s not such a youth, either.
- RU: «Его зовут Винсент Сполдинг, и он не такой уж и юноша.

**165. `h4-135`** · ch3 ¶37.2 · `ok` · status `silver`

- EN: It is not a common experience among employers in this age.
- RU: В наш век это нечастый опыт среди работодателей.

**166. `h4-136`** · ch3 ¶38.3 · `ok` · status `silver`

- EN: That is his main fault, but on the whole he’s a good worker.
- RU: Это его главный недостаток, но в целом он хороший работник.

**167. `h4-137`** · ch3 ¶41.0 · `ok` · status `silver`

- EN: “The first thing that put us out was that advertisement.
- RU: — Первое, что выбило нас из колеи, было то объявление.

**168. `h4-138`** · ch3 ¶44.2 · `ok` · status `silver`

- EN: If my hair would only change colour, here’s a nice little crib all ready for me to step into.
- RU: Если бы мои волосы только изменили цвет, вот отличное тёпленькое местечко, совсем готовое для меня».

**169. `h4-140`** · ch3 ¶51.0 · `ok` · status `silver`

- EN: “Well, you can easily think that that made me prick up my ears, for the business has not been over good for some years, and an extra couple of hundred would have been very handy.
- RU: — Ну, вы легко можете представить, что это заставило меня навострить уши, ведь дела уже несколько лет шли не слишком хорошо, и лишняя пара сотен была бы очень кстати.

**170. `h4-141`** · ch3 ¶53.3 · `ok` · status `silver`

- EN: From all I hear it is splendid pay and very little to do.’
- RU: Судя по всему, что я слышал, это великолепная оплата и очень мало работы».

**171. `h4-142`** · ch3 ¶55.3 · `ok` · status `silver`

- EN: Then, again, I have heard it is no use your applying if your hair is light red, or dark red, or anything but real bright, blazing, fiery red.
- RU: Затем, опять же, я слышал, что вам бесполезно обращаться, если ваши волосы светло-рыжие, или тёмно-рыжие, или какие-либо другие, кроме по-настоящему яркого, пылающего, огненно-рыжего цвета.

**172. `h4-144`** · ch3 ¶57.5 · `ok` · status `silver`

- EN: When I saw how many were waiting, I would have given it up in despair; but Spaulding would not hear of it.
- RU: Когда я увидел, сколько людей ждёт, я бы в отчаянии бросил это; но Сполдинг и слышать об этом не хотел.

**173. `h4-145`** · ch3 ¶59.0 · `ok` · status `silver`

- EN: “There was nothing in the office but a couple of wooden chairs and a deal table, behind which sat a small man with a head that was even redder than mine.
- RU: «В конторе не было ничего, кроме пары деревянных стульев и соснового стола, за которым сидел невысокий человечек с головой ещё более рыжей, чем моя.

**174. `h4-146`** · ch3 ¶60.0 · `ok` · status `silver`

- EN: “‘This is Mr. Jabez Wilson,’ said my assistant, ‘and he is willing to fill a vacancy in the League.’
- RU: «„Это мистер Джейбез Уилсон, — сказал мой помощник, — и он желает занять вакансию в Союзе“.

**175. `h4-148`** · ch3 ¶62.4 · `ok` · status `silver`

- EN: ‘I perceive that all is as it should be.
- RU: Я вижу, что всё так, как и должно быть.

**176. `h4-149`** · ch3 ¶63.0 · `ok` · status `silver`

- EN: “‘My name,’ said he, ‘is Mr. Duncan Ross, and I am myself one of the pensioners upon the fund left by our noble benefactor.
- RU: «Моё имя, — сказал он, — мистер Дункан Росс, и я сам являюсь одним из пенсионеров фонда, оставленного нашим благородным благодетелем.

**177. `h4-150`** · ch3 ¶66.1 · `ok` · status `silver`

- EN: ’ he said gravely, ‘that is very serious indeed!
- RU: — серьезно сказал он, — это действительно очень серьезно!

**178. `h4-151`** · ch3 ¶68.0 · `ok` · status `silver`

- EN: “‘In the case of another,’ said he, ‘the objection might be fatal, but we must stretch a point in favour of a man with such a head of hair as yours.
- RU: «В случае с другим, — сказал он, — это возражение могло бы быть роковым, но мы должны сделать уступку в пользу человека с такой шевелюрой, как ваша.

**179. `h4-152`** · ch3 ¶70.1 · `ok` · status `silver`

- EN: ’ said Vincent Spaulding.
- RU: — сказал Винсент Сполдинг.

**180. `h4-153`** · ch3 ¶74.0 · `ok` · status `silver`

- EN: “‘That would suit me very well,’ said I. ‘And the pay?’
- RU: «Это бы мне очень подошло, — сказал я. — А плата?»

**181. `h4-154`** · ch3 ¶79.2 · `ok` · status `silver`

- EN: The will is very clear upon that point.
- RU: Завещание очень четко говорит по этому поводу.

**182. `h4-155`** · ch3 ¶83.0 · `ok` · status `silver`

- EN: “‘Is to copy out the Encyclopædia Britannica.
- RU: — Состоит в том, чтобы переписывать «Британскую энциклопедию».

**183. `h4-156`** · ch3 ¶85.1 · `ok` · status `silver`

- EN: ’ He bowed me out of the room and I went home with my assistant, hardly knowing what to say or do, I was so pleased at my own good fortune.
- RU: » Он с поклоном выпроводил меня из комнаты, и я пошел домой со своим помощником, едва зная, что сказать или сделать, настолько я был обрадован своей удачей.

**184. `h4-157`** · ch3 ¶87.0 · `ok` · status `silver`

- EN: “Well, to my surprise and delight, everything was as right as possible.
- RU: — Ну, к моему удивлению и восторгу, все оказалось в полном порядке.

**185. `h4-158`** · ch3 ¶88.0 · `ok` · status `silver`

- EN: “This went on day after day, Mr. Holmes, and on Saturday the manager came in and planked down four golden sovereigns for my week’s work.
- RU: — Так продолжалось день за днем, мистер Холмс, а в субботу пришел управляющий и выложил четыре золотых соверена за мою недельную работу.

**186. `h4-159`** · ch3 ¶89.0 · `ok` · status `silver`

- EN: “Eight weeks passed away like this, and I had written about Abbots and Archery and Armour and Architecture and Attica, and hoped with diligence that I might get on to the B’s before very long.
- RU: — Так прошло восемь недель, и я написал об Аббатах, и о Стрельбе из лука, и о Доспехах, и об Архитектуре, и об Аттике, и надеялся, проявив усердие, довольно скоро добраться до буквы «Б».

**187. `h4-160`** · ch3 ¶91.3 · `ok` · status `silver`

- EN: Here it is, and you can read for yourself.”
- RU: Вот он, и вы можете прочесть сами».

**188. `h4-161`** · ch3 ¶95.0 · `ok` · status `silver`

- EN: “I cannot see that there is anything very funny,” cried our client, flushing up to the roots of his flaming head.
- RU: «Я не вижу здесь ничего особенно смешного», — воскликнул наш клиент, вспыхнув до самых корней своей пламенной шевелюры.

**189. `h4-162`** · ch3 ¶96.3 · `ok` · status `silver`

- EN: But there is, if you will excuse my saying so, something just a little funny about it.
- RU: Но во всем этом есть, если вы простите мне эти слова, нечто немного забавное.

**190. `h4-163`** · ch3 ¶97.3 · `ok` · status `silver`

- EN: Finally, I went to the landlord, who is an accountant living on the ground floor, and I asked him if he could tell me what had become of the Red-headed League.
- RU: Наконец, я пошел к домовладельцу, бухгалтеру, живущему на первом этаже, и спросил его, не может ли он сказать мне, что стало с Союзом рыжих.

**191. `h4-164`** · ch3 ¶98.0 · `ok` · status `silver`

- EN: “‘Well,’ said I, ‘the gentleman at No. 4.’
- RU: «— Ну, — сказал я, — джентльмен из номера четыре.

**192. `h4-165`** · ch3 ¶102.0 · `ok` · status `silver`

- EN: “‘Where could I find him?
- RU: «— Где я мог бы его найти?

**193. `h4-166`** · ch3 ¶105.0 · `ok` · status `silver`

- EN: “And what did you do then?” asked Holmes.
- RU: «И что вы сделали тогда?» — спросил Холмс.

**194. `h4-168`** · ch3 ¶108.1 · `ok` · status `silver`

- EN: “Why, I have lost four pound a week.”
- RU: — Как же, я потерял четыре фунта в неделю.

**195. `h4-169`** · ch3 ¶111.0 · `ok` · status `silver`

- EN: “We shall endeavour to clear up these points for you.
- RU: — Мы постараемся прояснить для вас эти моменты.

**196. `h4-170`** · ch3 ¶113.0 · `ok` · status `silver`

- EN: “How did he come?”
- RU: — Как он появился?

**197. `h4-171`** · ch3 ¶118.0 · `ok` · status `silver`

- EN: “Because he was handy and would come cheap.”
- RU: — Потому что он был толковым и соглашался на маленькое жалованье.

**198. `h4-172`** · ch3 ¶123.0 · `ok` · status `silver`

- EN: Holmes sat up in his chair in considerable excitement.
- RU: Холмс выпрямился в своем кресле в немалом волнении.

**199. `h4-173`** · ch3 ¶125.1 · `ok` · status `silver`

- EN: “He is still with you?”
- RU: «Он все еще у вас?»

**200. `h4-174`** · ch3 ¶129.0 · `ok` · status `silver`

- EN: “That will do, Mr. Wilson.
- RU: «Этого достаточно, мистер Уилсон.

**201. `h4-175`** · ch3 ¶131.1 · `ok` · status `silver`

- EN: “It is a most mysterious business.”
- RU: — Это чрезвычайно таинственное дело.

**202. `h4-176`** · ch3 ¶133.0 · `ok` · status `silver`

- EN: “What are you going to do, then?”
- RU: — Что же вы собираетесь делать?

**203. `h4-177`** · ch3 ¶135.0 · `ok` · status `silver`

- EN: “Sarasate plays at the St. James’s Hall this afternoon,” he remarked.
- RU: — Сегодня днём в Сент-Джеймс-холле играет Сарасате, — заметил он.

**204. `h4-178`** · ch3 ¶137.0 · `ok` · status `silver`

- EN: “Then put on your hat and come.
- RU: «Тогда надевайте шляпу и идемте.

**205. `h4-180`** · ch3 ¶138.6 · `ok` · status `silver`

- EN: It was instantly opened by a bright-looking, clean-shaven young fellow, who asked him to step in.
- RU: Ее мгновенно открыл смышленый на вид, чисто выбритый молодой человек, который предложил ему войти.

**206. `h4-181`** · ch3 ¶141.2 · `ok` · status `silver`

- EN: I have known something of him before.”
- RU: Я и прежде кое-что знал о нём.

**207. `h4-182`** · ch3 ¶146.0 · `ok` · status `silver`

- EN: “And what did you see?”
- RU: — И что же вы увидели?

**208. `h4-184`** · ch3 ¶150.3 · `ok` · status `silver`

- EN: It was difficult to realise as we looked at the line of fine shops and stately business premises that they really abutted on the other side upon the faded and stagnant square which we had just quitted.
- RU: Трудно было поверить, когда мы смотрели на ряд прекрасных магазинов и величественных конторских зданий, что они действительно примыкали с другой стороны к увядшей и застойной площади, которую мы только что покинули.

**209. `h4-185`** · ch3 ¶151.4 · `ok` · status `silver`

- EN: And now, Doctor, we’ve done our work, so it’s time we had some play.
- RU: А теперь, доктор, мы сделали нашу работу, так что пора нам немного развлечься.

**210. `h4-187`** · ch3 ¶155.0 · `ok` · status `silver`

- EN: “And I have some business to do which will take some hours.
- RU: — А у меня есть дело, которое займет несколько часов.

**211. `h4-188`** · ch3 ¶157.2 · `ok` · status `silver`

- EN: But to-day being Saturday rather complicates matters.
- RU: Но то, что сегодня суббота, несколько осложняет дело.

**212. `h4-189`** · ch3 ¶161.2 · `ok` · status `silver`

- EN: He waved his hand, turned on his heel, and disappeared in an instant among the crowd.
- RU: Он махнул рукой, повернулся на каблуках и в мгновение ока исчез в толпе.

**213. `h4-190`** · ch3 ¶162.4 · `ok` · status `silver`

- EN: Where were we going, and what were we to do?
- RU: Куда мы направлялись и что нам предстояло делать?

**214. `h4-191`** · ch3 ¶163.2 · `ok` · status `silver`

- EN: On entering his room, I found Holmes in animated conversation with two men, one of whom I recognised as Peter Jones, the official police agent, while the other was a long, thin, sad-faced man, with a very shiny hat and oppressively respectable frock-coat.
- RU: Войдя в его комнату, я застал Холмса за оживлённой беседой с двумя мужчинами, в одном из которых я узнал Питера Джонса, официального полицейского агента, тогда как другой был высоким, худым человеком с печальным лицом, в очень блестящем цилиндре и подавляюще респектабельном сюртуке.

**215. `h4-192`** · ch3 ¶165.1 · `ok` · status `silver`

- EN: “Our friend here is a wonderful man for starting a chase.
- RU: — Наш друг здесь — удивительный человек по части того, чтобы поднять дичь.

**216. `h4-194`** · ch3 ¶169.0 · `ok` · status `silver`

- EN: “I think you will find,” said Sherlock Holmes, “that you will play for a higher stake to-night than you have ever done yet, and that the play will be more exciting.
- RU: «Я полагаю, вы убедитесь, — сказал Шерлок Холмс, — что сегодня вечером будете играть по более высокой ставке, чем когда-либо прежде, и что игра будет более захватывающей.

**217. `h4-196`** · ch3 ¶171.1 · `ok` · status `silver`

- EN: I’ve had one or two little turns also with Mr. John Clay, and I agree with you that he is at the head of his profession.
- RU: У меня тоже было одно или два небольших столкновения с мистером Джоном Клеем, и я согласен с вами, что он стоит во главе своей профессии.

**218. `h4-197`** · ch3 ¶173.0 · `ok` · status `silver`

- EN: “We are close there now,” my friend remarked.
- RU: «Теперь мы уже близко», — заметил мой друг.

**219. `h4-198`** · ch3 ¶173.5 · `ok` · status `silver`

- EN: He is as brave as a bulldog and as tenacious as a lobster if he gets his claws upon anyone.
- RU: Он храбр, как бульдог, и цепок, как омар, если уж вцепится в кого-нибудь своими клешнями.

**220. `h4-200`** · ch3 ¶176.1 · `ok` · status `silver`

- EN: “Why, dear me, it sounds quite hollow!” he remarked, looking up in surprise.
- RU: — Батюшки, звук-то совершенно пустой! — заметил он, с удивлением подняв глаза.

**221. `h4-202`** · ch3 ¶180.0 · `ok` · status `silver`

- EN: “It is our French gold,” whispered the director.
- RU: — Это наше французское золото, — прошептал директор.

**222. `h4-203`** · ch3 ¶182.4 · `ok` · status `silver`

- EN: Our reserve of bullion is much larger at present than is usually kept in a single branch office, and the directors have had misgivings upon the subject.”
- RU: Наш запас золота в слитках в настоящее время гораздо больше, чем обычно хранится в одном отделении, и у директоров были опасения по этому поводу».

**223. `h4-204`** · ch3 ¶184.0 · `ok` · status `silver`

- EN: “And sit in the dark?”
- RU: «И сидеть в темноте?»

**224. `h4-205`** · ch3 ¶185.4 · `ok` · status `silver`

- EN: These are daring men, and though we shall take them at a disadvantage, they may do us some harm unless we are careful.
- RU: Это смелые люди, и хотя мы застигнем их врасплох, они могут причинить нам вред, если мы не будем осторожны.

**225. `h4-206`** · ch3 ¶186.0 · `ok` · status `silver`

- EN: I placed my revolver, cocked, upon the top of the wooden case behind which I crouched.
- RU: Я положил свой револьвер, взведённый, на верх деревянного ящика, за которым я присел.

**226. `h4-207`** · ch3 ¶187.1 · `ok` · status `silver`

- EN: “That is back through the house into Saxe-Coburg Square.
- RU: — Это обратно через дом на Сакс-Кобург-сквер.

**227. `h4-208`** · ch3 ¶190.0 · `ok` · status `silver`

- EN: What a time it seemed!
- RU: Каким долгим показалось это время!

**228. `h4-209`** · ch3 ¶191.0 · `ok` · status `silver`

- EN: At first it was but a lurid spark upon the stone pavement.
- RU: Сначала это была лишь зловещая искра на каменном полу.

**229. `h4-210`** · ch3 ¶192.1 · `ok` · status `silver`

- EN: With a rending, tearing sound, one of the broad, white stones turned over upon its side and left a square, gaping hole, through which streamed the light of a lantern.
- RU: С треском и скрежетом один из широких белых камней перевернулся набок и оставил квадратную зияющую дыру, сквозь которую струился свет фонаря.

**230. `h4-213`** · ch3 ¶198.1 · `ok` · status `silver`

- EN: You seem to have done the thing very completely.
- RU: Похоже, вы обделали дело на редкость основательно.

**231. `h4-215`** · ch3 ¶202.0 · `ok` · status `silver`

- EN: “All right,” said Jones with a stare and a snigger.
- RU: — Ладно, — сказал Джонс, уставившись на него и хихикнув.

**232. `h4-217`** · ch3 ¶206.2 · `ok` · status `silver`

- EN: The method was no doubt suggested to Clay’s ingenious mind by the colour of his accomplice’s hair.
- RU: Этот метод, без сомнения, был подсказан изобретательному уму Клея цветом волос его сообщника.

**233. `h4-218`** · ch3 ¶207.0 · `ok` · status `silver`

- EN: “But how could you guess what the motive was?”
- RU: — Но как вы могли догадаться, каков был этот мотив?

**234. `h4-219`** · ch3 ¶208.4 · `ok` · status `silver`

- EN: What could it be?
- RU: Что бы это могло быть?

**235. `h4-220`** · ch3 ¶208.10 · `ok` · status `silver`

- EN: What could it be, once more?
- RU: Что бы это могло быть, опять-таки?

**236. `h4-221`** · ch3 ¶209.3 · `ok` · status `silver`

- EN: It was not in front.
- RU: Он был не впереди.

**237. `h4-222`** · ch3 ¶209.8 · `ok` · status `silver`

- EN: You must yourself have remarked how worn, wrinkled, and stained they were.
- RU: Вы сами, должно быть, заметили, какими потертыми, мятыми и испачканными они были.

**238. `h4-223`** · ch3 ¶210.0 · `ok` · status `silver`

- EN: “And how could you tell that they would make their attempt to-night?”
- RU: «А как вы могли знать, что они предпримут свою попытку сегодня ночью?»

**239. `h4-224`** · ch3 ¶211.3 · `ok` · status `silver`

- EN: For all these reasons I expected them to come to-night.”
- RU: По всем этим причинам я ожидал, что они придут сегодня ночью».

**240. `h4-226`** · ch3 ¶215.2 · `ok` · status `silver`

- EN: “‘L’homme c’est rien—l’œuvre c’est tout,’ as Gustave Flaubert wrote to George Sand.”
- RU: «"L’homme c’est rien — l’œuvre c’est tout", как писал Гюстав Флобер Жорж Санд».

**241. `h4-227`** · ch4 ¶2.0 · `ok` · status `silver`

- EN: “And yet I am not convinced of it,” I answered.
- RU: «И всё же я не убеждён в этом», — ответил я.

**242. `h4-228`** · ch4 ¶3.2 · `ok` · status `silver`

- EN: Depend upon it, there is nothing so unnatural as the commonplace.”
- RU: Положитесь на это, нет ничего столь неестественного, как банальное».

**243. `h4-229`** · ch4 ¶4.4 · `ok` · status `silver`

- EN: Here is the first heading upon which I come.
- RU: Вот первый заголовок, на который я натолкнулся.

**244. `h4-231`** · ch4 ¶6.0 · `ok` · status `silver`

- EN: He held out his snuffbox of old gold, with a great amethyst in the centre of the lid.
- RU: Он протянул свою табакерку из старинного золота с большим аметистом в центре крышки.

**245. `h4-233`** · ch4 ¶11.2 · `ok` · status `silver`

- EN: Indeed, I have found that it is usually in unimportant matters that there is a field for the observation, and for the quick analysis of cause and effect which gives the charm to an investigation.
- RU: В самом деле, я обнаружил, что именно в неважных делах обычно находится поле для наблюдения и для быстрого анализа причин и следствий, что и придает расследованию всю прелесть.

**246. `h4-234`** · ch4 ¶12.1 · `ok` · status `silver`

- EN: Looking over his shoulder, I saw that on the pavement opposite there stood a large woman with a heavy fur boa round her neck, and a large curling red feather in a broad-brimmed hat which was tilted in a coquettish Duchess of Devonshire fashion over her ear.
- RU: Взглянув через его плечо, я увидел, что на противоположном тротуаре стояла крупная женщина с тяжелым меховым боа на шее и большим вьющимся красным пером на широкополой шляпе, которая была кокетливо сдвинута набок, на манер герцогини Девонширской.

**247. `h4-235`** · ch4 ¶13.2 · `ok` · status `silver`

- EN: She would like advice, but is not sure that the matter is not too delicate for communication.
- RU: Ей хотелось бы получить совет, но она не уверена, не слишком ли это дело деликатно для разглашения.

**248. `h4-236`** · ch4 ¶13.6 · `ok` · status `silver`

- EN: But here she comes in person to resolve our doubts.”
- RU: Но вот она идет лично, чтобы разрешить наши сомнения.

**249. `h4-238`** · ch4 ¶18.0 · `ok` · status `silver`

- EN: “I came to you, sir, because I heard of you from Mrs. Etherege, whose husband you found so easy when the police and everyone had given him up for dead.
- RU: — Я пришла к вам, сэр, потому что услышала о вас от миссис Этеридж, чьего мужа вы так легко нашли, когда полиция и все остальные уже посчитали его погибшим.

**250. `h4-240`** · ch4 ¶24.0 · `ok` · status `silver`

- EN: “Oh, yes, mother is alive and well.
- RU: — О да, мама жива и здорова.

**251. `h4-241`** · ch4 ¶26.0 · `ok` · status `silver`

- EN: “Your own little income,” he asked, “does it come out of the business?”
- RU: — Ваш собственный небольшой доход, — спросил он, — поступает от этого дела?

**252. `h4-242`** · ch4 ¶28.0 · `ok` · status `silver`

- EN: “You interest me extremely,” said Holmes.
- RU: — Вы чрезвычайно меня интересуете, — сказал Холмс.

**253. `h4-243`** · ch4 ¶29.2 · `ok` · status `silver`

- EN: Mr. Windibank draws my interest every quarter and pays it over to mother, and I find that I can do pretty well with what I earn at typewriting.
- RU: Мистер Виндибэнк забирает мои проценты каждый квартал и передает их матери, и я нахожу, что могу вполне неплохо обходиться тем, что зарабатываю машинописью.

**254. `h4-244`** · ch4 ¶31.0 · `ok` · status `silver`

- EN: A flush stole over Miss Sutherland’s face, and she picked nervously at the fringe of her jacket.
- RU: Румянец залил лицо мисс Сазерленд, и она нервно теребила бахрому своего жакета.

**255. `h4-245`** · ch4 ¶31.5 · `ok` · status `silver`

- EN: He would get quite mad if I wanted so much as to join a Sunday-school treat.
- RU: Он совершенно выходил из себя, если я хотела хотя бы пойти на праздник воскресной школы.

**256. `h4-246`** · ch4 ¶32.0 · `ok` · status `silver`

- EN: “I suppose,” said Holmes, “that when Mr. Windibank came back from France he was very annoyed at your having gone to the ball.”
- RU: — Я полагаю, — сказал Холмс, — что, когда мистер Виндибэнк вернулся из Франции, он был очень раздосадован тем, что вы пошли на бал.

**257. `h4-247`** · ch4 ¶37.0 · `ok` · status `silver`

- EN: “Well, you know father didn’t like anything of the sort.
- RU: — Ну, знаете, отец не любил ничего подобного.

**258. `h4-248`** · ch4 ¶38.1 · `ok` · status `silver`

- EN: Did he make no attempt to see you?”
- RU: Неужели он не делал попыток увидеться с вами?

**259. `h4-249`** · ch4 ¶41.0 · `ok` · status `silver`

- EN: “Oh, yes, Mr. Holmes.
- RU: — О да, мистер Холмс.

**260. `h4-251`** · ch4 ¶49.1 · `ok` · status `silver`

- EN: He said that if they were sent to the office he would be chaffed by all the other clerks about having letters from a lady, so I offered to typewrite them, like he did his, but he wouldn’t have that, for he said that when I wrote them they seemed to come from me, but when they were typewritten he always felt that the machine had come between us.
- RU: Он говорил, что если их присылать в контору, все остальные клерки стали бы подшучивать над ним из-за писем от дамы, поэтому я предложила печатать их на машинке, как делал он со своими, но он не согласился на это, так как сказал, что, когда я писала их от руки, они словно исходили от меня, а когда они были напечатаны, он всегда чувствовал, что между нами встала машинка.

**261. `h4-252`** · ch4 ¶51.0 · `ok` · status `silver`

- EN: “He was a very shy man, Mr. Holmes.
- RU: — Он был очень робким человеком, мистер Холмс.

**262. `h4-254`** · ch4 ¶53.2 · `ok` · status `silver`

- EN: Mother said he was quite right to make me swear, and that it was a sign of his passion.
- RU: Мама сказала, что он совершенно прав, заставляя меня поклясться, и что это признак его страсти.

**263. `h4-255`** · ch4 ¶54.0 · `ok` · status `silver`

- EN: “It missed him, then?”
- RU: — Значит, оно разминулось с ним?

**264. `h4-256`** · ch4 ¶57.0 · `ok` · status `silver`

- EN: “Yes, sir, but very quietly.
- RU: — Да, сэр, но очень тихо.

**265. `h4-257`** · ch4 ¶57.5 · `ok` · status `silver`

- EN: That was last Friday, Mr. Holmes, and I have never seen or heard anything since then to throw any light upon what became of him.”
- RU: Это было в прошлую пятницу, мистер Холмс, и с тех пор я не видела и не слышала ничего, что пролило бы какой-либо свет на то, что с ним сталось».

**266. `h4-258`** · ch4 ¶60.0 · `ok` · status `silver`

- EN: “Most certainly it does.
- RU: «Вне всякого сомнения.

**267. `h4-261`** · ch4 ¶67.6 · `ok` · status `silver`

- EN: She pulled a little handkerchief out of her muff and began to sob heavily into it.
- RU: Она вытащила маленький носовой платок из своей муфты и принялась тяжело рыдать в него.

**268. `h4-262`** · ch4 ¶71.0 · `ok` · status `silver`

- EN: “Then what has happened to him?”
- RU: — Что же тогда с ним случилось?

**269. `h4-263`** · ch4 ¶75.0 · `ok` · status `silver`

- EN: “No. 31 Lyon Place, Camberwell.”
- RU: «Лайон-плейс, 31, Кэмберуэлл».

**270. `h4-264`** · ch4 ¶78.2 · `ok` · status `silver`

- EN: You will leave the papers here, and remember the advice which I have given you.
- RU: Оставьте бумаги здесь и помните совет, который я вам дал.

**271. `h4-265`** · ch4 ¶80.0 · `ok` · status `silver`

- EN: For all the preposterous hat and the vacuous face, there was something noble in the simple faith of our visitor which compelled our respect.
- RU: Несмотря на всю нелепость шляпки и пустоту лица, в простодушной вере нашей посетительницы было нечто благородное, внушавшее уважение.

**272. `h4-266`** · ch4 ¶82.0 · `ok` · status `silver`

- EN: “Quite an interesting study, that maiden,” he observed.
- RU: — Весьма занятный предмет для изучения, эта девица, — заметил он.

**273. `h4-267`** · ch4 ¶83.0 · `ok` · status `silver`

- EN: “You appeared to read a good deal upon her which was quite invisible to me,” I remarked.
- RU: — Вы, казалось, прочли по ней очень многое, что было совершенно невидимо для меня, — заметил я.

**274. `h4-268`** · ch4 ¶85.0 · `ok` · status `silver`

- EN: “Well, she had a slate-coloured, broad-brimmed straw hat, with a feather of a brickish red.
- RU: — Ну, на ней была грифельного цвета широкополая соломенная шляпка с кирпично-красным пером.

**275. `h4-270`** · ch4 ¶87.3 · `ok` · status `silver`

- EN: Never trust to general impressions, my boy, but concentrate yourself upon details.
- RU: Никогда не полагайтесь на общие впечатления, мой мальчик, а сосредоточивайтесь на деталях.

**276. `h4-271`** · ch4 ¶87.8 · `ok` · status `silver`

- EN: The sewing-machine, of the hand type, leaves a similar mark, but only on the left arm, and on the side of it farthest from the thumb, instead of being right across the broadest part, as this was.
- RU: Швейная машинка ручного типа оставляет похожий след, но только на левой руке и на той её стороне, что дальше всего от большого пальца, вместо того чтобы проходить прямо по самой широкой части, как было здесь.

**277. `h4-272`** · ch4 ¶89.2 · `ok` · status `silver`

- EN: One was buttoned only in the two lower buttons out of five, and the other at the first, third, and fifth.
- RU: Один был застёгнут только на две нижние пуговицы из пяти, а другой — на первую, третью и пятую.

**278. `h4-274`** · ch4 ¶92.1 · `ok` · status `silver`

- EN: “Missing,” it said, “on the morning of the fourteenth, a gentleman named Hosmer Angel.
- RU: — «Пропал, — гласила она, — утром четырнадцатого числа джентльмен по имени Госмер Эйнджел.

**279. `h4-275`** · ch4 ¶92.6 · `ok` · status `silver`

- EN: Anybody bringing,” &c, &c.
- RU: Всякому, кто доставит...» и так далее, и тому подобное.

**280. `h4-276`** · ch4 ¶94.0 · `ok` · status `silver`

- EN: “They are typewritten,” I remarked.
- RU: — Они напечатаны на машинке, — заметил я.

**281. `h4-277`** · ch4 ¶97.0 · `ok` · status `silver`

- EN: “My dear fellow, is it possible you do not see how strongly it bears upon the case?”
- RU: — Дорогой мой друг, неужели вы не видите, насколько сильно это относится к делу?

**282. `h4-278`** · ch4 ¶99.2 · `ok` · status `silver`

- EN: One is to a firm in the City, the other is to the young lady’s stepfather, Mr. Windibank, asking him whether he could meet us here at six o’clock to-morrow evening.
- RU: Одно — в фирму в Сити, другое — отчиму барышни, мистеру Виндибэнку, с вопросом, сможет ли он встретиться с нами здесь завтра вечером в шесть часов.

**283. `h4-280`** · ch4 ¶103.0 · `ok` · status `silver`

- EN: “Well, have you solved it?”
- RU: — Ну что, вы разрешили её?

**284. `h4-281`** · ch4 ¶106.2 · `ok` · status `silver`

- EN: There was never any mystery in the matter, though, as I said yesterday, some of the details are of interest.
- RU: В этом деле никогда не было никакой тайны, хотя, как я и сказал вчера, некоторые детали представляют интерес.

**285. `h4-282`** · ch4 ¶109.1 · `ok` · status `silver`

- EN: “He has written to me to say that he would be here at six.
- RU: — Он написал мне, что будет здесь в шесть часов.

**286. `h4-283`** · ch4 ¶112.1 · `ok` · status `silver`

- EN: I am afraid that I am a little late, but I am not quite my own master, you know.
- RU: Боюсь, что я немного опоздал, но, знаете ли, я не вполне сам себе хозяин.

**287. `h4-284`** · ch4 ¶112.5 · `ok` · status `silver`

- EN: Besides, it is a useless expense, for how could you possibly find this Hosmer Angel?”
- RU: Кроме того, это бесполезные расходы, ведь как вы вообще могли бы найти этого Госмера Эйнджела?»

**288. `h4-285`** · ch4 ¶115.1 · `ok` · status `silver`

- EN: Unless they are quite new, no two of them write exactly alike.
- RU: Если только они не совсем новые, нет двух таких, которые печатали бы совершенно одинаково.

**289. `h4-286`** · ch4 ¶117.0 · `ok` · status `silver`

- EN: “And now I will show you what is really a very interesting study, Mr. Windibank,” Holmes continued.
- RU: «А теперь я покажу вам то, что действительно представляет собой очень интересное исследование, мистер Виндибэнк», — продолжил Холмс.

**290. `h4-287`** · ch4 ¶117.5 · `ok` · status `silver`

- EN: In each case, not only are the ‘e’s’ slurred and the ‘r’s’ tailless, but you will observe, if you care to use my magnifying lens, that the fourteen other characteristics to which I have alluded are there as well.”
- RU: В каждом случае не только смазаны «e» и лишены хвостиков «r», но вы также заметите, если соблаговолите воспользоваться моей увеличительной лупой, что остальные четырнадцать признаков, о которых я упоминал, тоже налицо».

**291. `h4-288`** · ch4 ¶119.1 · `ok` · status `silver`

- EN: “I let you know, then, that I have caught him!”
- RU: — Тогда я даю вам знать, что я поймал его!

**292. `h4-289`** · ch4 ¶121.4 · `ok` · status `silver`

- EN: Sit down and let us talk it over.”
- RU: Садитесь и давайте все обсудим.

**293. `h4-290`** · ch4 ¶123.1 · `ok` · status `silver`

- EN: But between ourselves, Windibank, it was as cruel and selfish and heartless a trick in a petty way as ever came before me.
- RU: Но между нами, Виндибэнк, это была самая жестокая, эгоистичная и бессердечная проделка в своем мелком роде, с какой мне когда-либо приходилось сталкиваться.

**294. `h4-292`** · ch4 ¶125.6 · `ok` · status `silver`

- EN: But soon he found that that would not answer forever.
- RU: Но вскоре он понял, что так не может продолжаться вечно.

**295. `h4-293`** · ch4 ¶126.0 · `ok` · status `silver`

- EN: “It was only a joke at first,” groaned our visitor.
- RU: — Сначала это была всего лишь шутка, — простонал наш посетитель.

**296. `h4-295`** · ch4 ¶127.9 · `ok` · status `silver`

- EN: James Windibank wished Miss Sutherland to be so bound to Hosmer Angel, and so uncertain as to his fate, that for ten years to come, at any rate, she would not listen to another man.
- RU: Джеймс Уиндибенк желал, чтобы мисс Сазерленд была настолько привязана к Госмеру Эйнджелу и пребывала в такой неуверенности относительно его судьбы, чтобы по крайней мере в течение следующих десяти лет она и слушать не стала другого мужчину.

**297. `h4-296`** · ch4 ¶129.0 · `ok` · status `silver`

- EN: “It may be so, or it may not, Mr. Holmes,” said he, “but if you are so very sharp you ought to be sharp enough to know that it is you who are breaking the law now, and not me. I have done nothing actionable from the first, but as long as you keep that door locked you lay yourself open to an action for assault and illegal constraint.”
- RU: — Может быть так, а может и нет, мистер Холмс, — сказал он, — но если вы настолько проницательны, вам следовало бы быть достаточно сообразительным, чтобы знать, что именно вы сейчас нарушаете закон, а не я. Я с самого начала не совершил ничего наказуемого, но пока вы держите эту дверь запертой, вы навлекаете на себя иск за нападение и незаконное лишение свободы.

**298. `h4-297`** · ch4 ¶131.0 · `ok` · status `silver`

- EN: “There’s a cold-blooded scoundrel!” said Holmes, laughing, as he threw himself down into his chair once more.
- RU: «Вот ведь хладнокровный негодяй!» — сказал Холмс, смеясь и снова опускаясь в свое кресло.

**299. `h4-298`** · ch4 ¶133.1 · `ok` · status `silver`

- EN: Then the fact that the two men were never together, but that the one always appeared when the other was away, was suggestive.
- RU: Затем тот факт, что эти двое мужчин никогда не появлялись вместе, но один всегда возникал, когда другого не было, наводил на размышления.

**300. `h4-300`** · ch4 ¶135.5 · `ok` · status `silver`

- EN: As I expected, his reply was typewritten and revealed the same trivial but characteristic defects.
- RU: Как я и ожидал, его ответ был напечатан на машинке и обнаружил те же мелкие, но характерные дефекты.

