#!/usr/bin/env python3
"""Gender agreement around character names: mine the fact, then audit the output.

--mine   derive each name's gender from the SOURCE side (he/his/him vs she/her
         in the same sentence, plus honorifics) — the fact a per-sentence
         translator cannot see and the glossary could carry.
--audit  find sentences where a name is the subject of a past-tense verb and
         read, through the stored alignment, whether the Russian verb agrees
         with that gender. Strict subject test (name immediately before the
         verb) keeps precision high at the cost of recall.

Usage: gender_probe.py BOOK.tbook --mine --out G.json
       gender_probe.py BOOK.tbook --audit G.json [--lang ru]
"""
import argparse, json, re, sys
from collections import Counter, defaultdict

sys.path.insert(0, __file__.rsplit("/", 1)[0])
from glossary_demand import POSS, STOP, WORD, sentences

PRON_M = {"he", "him", "his", "himself"}
PRON_F = {"she", "her", "hers", "herself"}
HON_M = {"mr", "mister", "sir", "lord", "father", "brother", "uncle", "son"}
HON_F = {"ms", "mrs", "miss", "madam", "lady", "mother", "sister", "aunt", "daughter"}
IRREG = {
    "said": "say", "told": "tell", "went": "go", "came": "come", "saw": "see",
    "took": "take", "gave": "give", "got": "get", "made": "make", "knew": "know",
    "thought": "think", "found": "find", "felt": "feel", "left": "leave",
    "held": "hold", "kept": "keep", "sat": "sit", "stood": "stand", "ran": "run",
    "began": "begin", "broke": "break", "brought": "bring", "caught": "catch",
    "chose": "choose", "did": "do", "drew": "draw", "drove": "drive", "fell": "fall",
    "flew": "fly", "forgot": "forget", "grew": "grow", "heard": "hear", "hit": "hit",
    "led": "lead", "lost": "lose", "meant": "mean", "met": "meet", "paid": "pay",
    "put": "put", "read": "read", "rose": "rise", "sent": "send", "shook": "shake",
    "shot": "shoot", "shut": "shut", "sold": "sell", "spoke": "speak", "spent": "spend",
    "swung": "swing", "threw": "throw", "understood": "understand", "wore": "wear",
    "woke": "wake", "wrote": "write", "lay": "lie", "let": "let", "cut": "cut",
    "hung": "hang", "slid": "slide", "spun": "spin", "struck": "strike", "swept": "sweep",
    "tore": "tear", "won": "win", "leaned": "lean", "was": "be", "had": "have",
    "hurt": "hurt", "beat": "beat", "bent": "bend", "bled": "bleed", "burst": "burst",
    "dealt": "deal", "dug": "dig", "drank": "drink", "fought": "fight", "fled": "flee",
    "hid": "hide", "knelt": "kneel", "lit": "light", "meant": "mean", "rode": "ride",
    "rang": "ring", "sang": "sing", "sank": "sink", "sought": "seek", "shrugged": "shrug",
    "slept": "sleep", "slipped": "slip", "spoilt": "spoil", "sprang": "spring",
    "stole": "steal", "stuck": "stick", "stung": "sting", "swore": "swear", "swam": "swim",
    "taught": "teach", "tossed": "toss", "wound": "wind", "wept": "weep", "withdrew": "withdraw",
}
# Russian past tense: -л / -ла / -ло / -ли (+ reflexive -ся/-сь). Restricting the
# stem vowel keeps nouns like «стол», «отдел», «зеркала» out.
PAST = re.compile(r"^(?P<stem>.*[аяеиыл우у]?)(?P<end>"
                  r"ал|ял|ел|ил|ыл|ул|ол|ёл|шёл|шел|"
                  r"ала|яла|ела|ила|ыла|ула|ола|шла|"
                  r"ало|яло|ело|ило|ыло|уло|шло|"
                  r"али|яли|ели|или|ыли|ули|шли)(?:ся|сь)?$")
FEM_END = ("ала", "яла", "ела", "ила", "ыла", "ула", "ола", "шла")
PL_END = ("али", "яли", "ели", "или", "ыли", "ули", "шли")
NEU_END = ("ало", "яло", "ело", "ило", "ыло", "уло", "шло")
TOKEN = re.compile(r"[^\W\d_]+", re.UNICODE)


def past_gender(tok):
    """m | f | n | pl | None for a Russian past-tense form."""
    w = tok.lower().replace("ё", "ё")
    m = PAST.match(w)
    if not m or len(w) < 4:
        return None
    e = m.group("end")
    if e in FEM_END:
        return "f"
    if e in PL_END:
        return "pl"
    if e in NEU_END:
        return "n"
    return "m"


def is_past_en(w):
    lw = w.lower()
    return lw in IRREG or (lw.endswith("ed") and len(lw) > 4)


def mine(book, lang, min_evidence):
    """Gender evidence from the source, using only patterns that bind to ONE name.

    Counting he/she anywhere in the sentence is not enough — a sentence usually
    holds several characters, and that noise put Bancroft (male) at 0.52 female.
    Two binding patterns instead: an honorific directly before the name, and the
    first gendered pronoun after the name with no other name in between.
    """
    ev = defaultdict(Counter)
    freq = Counter()
    for s, _ in sentences(book, lang):
        src = s["src"]
        toks = [m.group(0) for m in WORD.finditer(src)]
        low = [t.lower() for t in toks]
        isname = []
        for i, raw in enumerate(toks):
            w = POSS.sub("", raw)
            # A name at the start of a sentence is evidence too: the candidate
            # miner must ignore sentence-initial capitals, but gender mining runs
            # over names that are already established, and most narration puts
            # the name first ("Ortega lifted her hand.").
            nm = (w[:1].isupper() and len(w) >= 3 and w.lower() not in STOP
                  and not w.startswith(("I’", "I'")) and w != "I")
            isname.append(w if nm else None)
            if nm:
                freq[w] += 1
        for i, name in enumerate(isname):
            if not name:
                continue
            if i and low[i - 1] in HON_M:
                ev[name]["m"] += 3
            elif i and low[i - 1] in HON_F:
                ev[name]["f"] += 3
            for j in range(i + 1, len(low)):
                if isname[j] and isname[j] != name:
                    break                      # another character intervenes
                if low[j] in PRON_M:
                    ev[name]["m"] += 1
                    break
                if low[j] in PRON_F:
                    ev[name]["f"] += 1
                    break
    out = {}
    for n, c in ev.items():
        tot = c["m"] + c["f"]
        if tot < min_evidence or freq[n] < 4:
            continue
        g = "m" if c["m"] > c["f"] else "f"
        out[n] = {"gender": g, "confidence": round(max(c["m"], c["f"]) / tot, 3),
                  "evidence": tot, "freq": freq[n]}
    return out


def audit(book, lang, table, window):
    """Count agreement on pairs where the name is unambiguously the subject.

    The loose version of this test (any past verb within 3 words) is ~90% false
    positives: a vocative («мистер Ковач?» — спросила она) or a possessive
    (Ortega's gang sat) puts a name next to a verb that belongs to someone else.
    The strict test below — name directly followed by the verb, no possessive,
    no comma after the name — trades recall for a rate that can be trusted.
    """
    hits, errs = Counter(), []
    checked = []
    for s, tr in sentences(book, lang):
        if not tr or not tr.get("align"):
            continue
        src, words, text = s["src"], s.get("words") or [], tr["text"]
        base, poss, comma = [], [], []
        for b, e in words:
            raw = src[b:e]
            m = WORD.search(raw)
            base.append(POSS.sub("", m.group(0)) if m else "")
            poss.append(bool(POSS.search(m.group(0))) if m else False)
            comma.append(raw.endswith((",", ":", ";", "?", "!", ".", "”", "\"")) or
                         src[e:e + 1] in (",", ":", ";", "?", "!", "”", "\""))
        for i, w in enumerate(base):
            info = table.get(w)
            if not info or poss[i] or comma[i]:
                continue             # possessive or vocative: not the subject
            for j in range(i + 1, min(i + 1 + window, len(base))):
                if not is_past_en(base[j]):
                    continue
                if any(base[k].lower() in ("and", "or", "who", "that", "which", "he",
                                           "she", "it", "they", "we", "i", "you")
                       or table.get(base[k]) for k in range(i + 1, j)):
                    break            # subject may have changed
                got = None
                for ch in tr["align"]:
                    if j in (ch.get("w") or []):
                        for t in TOKEN.findall(text[ch["t"][0]:ch["t"][1]]):
                            g = past_gender(t)
                            if g:
                                got = (g, t)
                                break
                    if got:
                        break
                if not got:
                    break
                g, form = got
                want = info["gender"]
                checked.append((w, want, g, form))
                hits[f"{want}->{g}"] += 1
                if g in ("m", "f") and g != want:
                    errs.append({"name": w, "want": want, "got": g, "form": form,
                                 "src": src, "tr": text})
                break
    return hits, errs, checked


def build_probe(book, lang, table, max_len=200):
    """Sentences where a known-gender name is the subject of a past-tense verb
    AND the source itself does not reveal the gender — the only case where a
    glossary tag can decide anything the sentence cannot."""
    out = []
    for s, tr in sentences(book, lang):
        src = s["src"]
        if not (40 <= len(src) <= max_len):  # noqa: E501
            continue
        toks = [m.group(0) for m in WORD.finditer(src)]
        low = [t.lower() for t in toks]
        if set(low) & (PRON_M | PRON_F):
            continue                                     # source reveals gender
        if sum(1 for t in low if is_past_en(t)) != 1:
            continue                                     # keep the check unambiguous
        base = [POSS.sub("", t) for t in toks]
        for i, w in enumerate(base):
            if w not in table or POSS.search(toks[i]):
                continue
            m = re.search(r"(?<![^\W_])" + re.escape(w) + r"(?![^\W_])", src)
            if m and src[m.end():m.end() + 1] in (",", ":", ";", "?", "!"):
                continue                                 # vocative
            nxt = next((j for j in range(i + 1, min(i + 3, len(base)))
                        if is_past_en(base[j])), None)
            if nxt is not None and all(not table.get(base[k]) and low[k] not in
                                       ("and", "or", "who", "that", "which")
                                       for k in range(i + 1, nxt)):
                out.append({"i": len(out), "src": src, "name": w,
                            "gender": table[w], "ref": (tr or {}).get("text"),
                            "stratum": "gender"})
                break
    return out


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("book"); ap.add_argument("--lang", default="ru")
    ap.add_argument("--mine", action="store_true")
    ap.add_argument("--audit"); ap.add_argument("--out")
    ap.add_argument("--min-evidence", type=int, default=3)
    ap.add_argument("--window", type=int, default=3)
    ap.add_argument("--probe", help="glossary with gender tags -> build a probe set")
    a = ap.parse_args()

    if a.probe:
        gl = json.load(open(a.probe))
        table = {e["src"]: e["gender"] for e in gl if e.get("gender")}
        probe = build_probe(a.book, a.lang, table)
        from collections import Counter as C
        print(f"gender probe: {len(probe)} sentences over "
              f"{len(set(p['name'] for p in probe))} names "
              f"(m {sum(1 for p in probe if p['gender']=='m')} / "
              f"f {sum(1 for p in probe if p['gender']=='f')})")
        print("  top names:", ", ".join(f"{n}×{c}" for n, c in
              C(p["name"] for p in probe).most_common(10)))
        if a.out:
            json.dump(probe, open(a.out, "w"), ensure_ascii=False, indent=1)
        return

    if a.mine:
        t = mine(a.book, a.lang, a.min_evidence)
        print(f"mined gender for {len(t)} names")
        for n, i in sorted(t.items(), key=lambda kv: -kv[1]["freq"])[:30]:
            print(f"  {n:<14} {i['gender']}  conf {i['confidence']:.2f}  "
                  f"evidence {i['evidence']:>3}  freq {i['freq']:>4}")
        low = [n for n, i in t.items() if i["confidence"] < 0.8]
        print(f"  low-confidence (<0.8): {len(low)} — {', '.join(low[:15])}")
        if a.out:
            json.dump(t, open(a.out, "w"), ensure_ascii=False, indent=1)
    if a.audit:
        table = json.load(open(a.audit))
        table = {k: v for k, v in table.items() if v["confidence"] >= 0.8}
        hits, errs, checked = audit(a.book, a.lang, table, a.window)
        n = sum(hits.values())
        wrong = sum(v for k, v in hits.items() if k in ("m->f", "f->m"))
        print(f"\naudited {n} name+past-verb pairs on {len(table)} confident names")
        for k, v in sorted(hits.items(), key=lambda kv: -kv[1]):
            print(f"  {k:>8}: {v}")
        print(f"WRONG GENDER: {wrong} ({100*wrong/max(1,n):.1f}% of audited pairs)")
        by = Counter(e["name"] for e in errs)
        print("  by name:", ", ".join(f"{k}×{v}" for k, v in by.most_common(12)))
        for e in errs[:12]:
            print(f"\n  {e['name']} ({e['want']}) got «{e['form']}»")
            print(f"    EN: {e['src'][:150]}")
            print(f"    RU: {e['tr'][:150]}")
        if a.out:
            json.dump(errs, open(a.out, "w"), ensure_ascii=False, indent=1)


if __name__ == "__main__":
    main()
