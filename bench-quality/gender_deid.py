#!/usr/bin/env python3
"""De-identify the gender probe so world knowledge cannot answer it.

On a famous novel the glossary doubles as a book identifier: a cast list of
Kovacs / Ortega / Kawahara lets the model recall from its own knowledge that
Ortega is a woman, and it then agrees correctly with no gender tag at all. That
makes the tag untestable. Here every character name in the probe (and in the
glossary) is replaced by an invented surname with a consonant-final Russian
rendering, so nothing but an explicit tag can carry the gender.

Usage: gender_deid.py PROBE.json GLOSSARY.json --out-probe P --out-gloss G
"""
import argparse, json, re, sys

# Invented surnames, all consonant-final in Russian so the rendering itself
# signals no gender.
PSEUDO = [
    ("Anverst", "Анверст"), ("Belforth", "Белфорт"), ("Crandell", "Крэнделл"),
    ("Vantrell", "Вантрелл"), ("Brantiss", "Брантисс"), ("Selvick", "Селвик"),
    ("Torvann", "Торванн"), ("Hesperin", "Хесперин"), ("Quillard", "Куиллард"),
    ("Draymont", "Дреймонт"), ("Faskell", "Фаскелл"), ("Nurvek", "Нурвек"),
    ("Palvrent", "Палврент"), ("Cormish", "Кормиш"), ("Grimsell", "Гримселл"),
    ("Halvorn", "Халворн"), ("Iskarn", "Искарн"), ("Jennarik", "Дженнарик"),
    ("Kelvarn", "Келварн"), ("Lomrith", "Ломрит"), ("Merridge", "Мерридж"),
    ("Norvell", "Норвелл"), ("Ostrand", "Острэнд"), ("Pentrell", "Пентрелл"),
    ("Quarrick", "Куоррик"), ("Rendisk", "Рендиск"), ("Sarnell", "Сарнелл"),
    ("Tarquist", "Тарквист"), ("Umbrell", "Умбрелл"), ("Vorsted", "Ворстед"),
    ("Wexmoor", "Уэксмур"), ("Yarnick", "Ярник"), ("Zalbrek", "Залбрек"),
    ("Cavrin", "Каврин"), ("Dunwold", "Данволд"), ("Eskvale", "Эсквейл"),
    ("Fennick", "Фенник"), ("Gorlund", "Горлунд"), ("Harnwell", "Харнвелл"),
    ("Ilverston", "Илверстон"), ("Jarmond", "Джармонд"), ("Kestrand", "Кестранд"),
    ("Lorvick", "Лорвик"), ("Mundrell", "Мандрелл"), ("Nesbrand", "Несбранд"),
    ("Oakvern", "Оукверн"), ("Prendish", "Прендиш"), ("Quorrell", "Кворрелл"),
    ("Ravensk", "Равенск"), ("Solvarn", "Солварн"), ("Thorbeck", "Торбек"),
    ("Ulmstead", "Улмстед"), ("Vandrick", "Вандрик"), ("Wexlin", "Уэкслин"),
    ("Xanwell", "Ксанвелл"), ("Yorbeck", "Йорбек"), ("Zennard", "Зеннард"),
    ("Ashvern", "Эшверн"), ("Brackwell", "Брэквелл"), ("Corvant", "Корвант"),
    ("Delmerick", "Делмерик"), ("Estrand", "Эстранд"), ("Follick", "Фоллик"),
    ("Garveth", "Гарвет"), ("Holstrom", "Холстром"), ("Invarn", "Инварн"),
    ("Jaskell", "Джаскелл"), ("Kolvern", "Колверн"), ("Lundrith", "Лундрит"),
    ("Marvant", "Марвант"), ("Nordwick", "Нордвик"), ("Ovrell", "Оврелл"),
    ("Purvane", "Пурвейн"), ("Rathwell", "Ратвелл"), ("Stromvik", "Стромвик"),
]


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("probe"); ap.add_argument("glossary")
    ap.add_argument("--out-probe", required=True); ap.add_argument("--out-gloss", required=True)
    a = ap.parse_args()

    probe = json.load(open(a.probe))
    gloss = json.load(open(a.glossary))

    # Every name that occurs in a probe sentence must be replaced, not just the
    # tested one — one surviving real name re-identifies the book.
    names = sorted({e["src"] for e in gloss} | {p["name"] for p in probe})
    present = [n for n in names
               if any(re.search(r"(?<![^\W_])" + re.escape(n) + r"(?![^\W_])", p["src"])
                      for p in probe)]
    if len(present) > len(PSEUDO):
        sys.exit(f"need {len(present)} pseudonyms, have {len(PSEUDO)}")
    amap = {n: PSEUDO[i] for i, n in enumerate(sorted(present))}
    print(f"{len(amap)} names replaced; tested names: "
          f"{len({p['name'] for p in probe})}")

    def deid(text):
        for n, (en, _) in amap.items():
            text = re.sub(r"(?<![^\W_])" + re.escape(n) + r"(?![^\W_])", en, text)
        return text

    out_probe = []
    for p in probe:
        if p["name"] not in amap:
            continue
        out_probe.append({**p, "src": deid(p["src"]), "name": amap[p["name"]][0],
                          "orig_name": p["name"], "ref": None})
    gender_of = {p["name"]: p["gender"] for p in out_probe}
    out_gloss = []
    for n, (en, ru) in sorted(amap.items(), key=lambda kv: kv[1][0]):
        e = {"src": en, "tgt": ru}
        if en in gender_of:
            e["gender"] = gender_of[en]
        out_gloss.append(e)
    json.dump(out_probe, open(a.out_probe, "w"), ensure_ascii=False, indent=1)
    json.dump(out_gloss, open(a.out_gloss, "w"), ensure_ascii=False, indent=1)
    print(f"probe {len(out_probe)} sentences (m {sum(1 for p in out_probe if p['gender']=='m')} / "
          f"f {sum(1 for p in out_probe if p['gender']=='f')}), "
          f"glossary {len(out_gloss)} entries, {sum(1 for e in out_gloss if 'gender' in e)} with gender")
    print("example:", out_probe[0]["src"][:120])


if __name__ == "__main__":
    main()
