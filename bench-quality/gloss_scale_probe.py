#!/usr/bin/env python3
"""Glossary-scale probe: render a large glossary, then translate with prefixes of it.

Speaks the SAME contract as the production translate pass (internal/translate:
prompt.go + client.go) — same system prompt, same {id,src} user JSON, same
temperature / json_object / reasoning-off request — so numbers transfer. The
prompt template is checked byte-for-byte against a Go-rendered dump.

  --check                     verify prompt rendering against the Go dump
  --render CAND.json          filter+render mined candidates into a glossary
  --translate PROBE.json      translate a probe set with --gloss-size N entries
"""
import argparse, json, os, re, sys, time, urllib.error, urllib.request
from concurrent.futures import ThreadPoolExecutor

HERE = os.path.dirname(os.path.abspath(__file__))
ART = os.path.join(HERE, ".artifacts", "glossary-scale")

LANG = {"en": "English", "ru": "Russian", "es": "Spanish", "de": "German"}

# Verbatim from internal/translate/prompt.go (translateSystemPrompt); the
# --check mode diffs the rendering against a Go dump of the same function.
TRANSLATE_TMPL = """You translate {SRC} → {TGT} for a language-learning reader: the app shows your
translation beside the original and highlights matching word pairs, so the reader
constantly compares the two texts word by word.

You receive a JSON array of sentences, each {id, src}.

For EACH sentence, write a faithful, natural literary {TGT} translation of src:
- Each sentence STANDS ALONE: translate exactly the content of its own src — never
  borrow, merge, or shift words or meaning from a neighboring sentence in the batch.
- COMPLETE and EXACT: every meaning element of src appears in the translation — no
  dropped clause, modifier, or negation; nothing invented; numbers, names, and
  quoted speech preserved.
- Natural {TGT} comes first — but when two renderings are equally natural, prefer
  the one that MIRRORS the source: give each content word an explicit {TGT}
  counterpart, keep the source clause order, keep metaphors as images. Do not
  paraphrase freely; split or merge clauses only where {TGT} grammar demands it.
- Match the source register and tone: slang stays slang, formal stays formal.
- Output PURE {TGT} — never leave {SRC} words in the translation.

Reply with ONLY a single JSON object mapping each "id" (exact string) to its {TGT}
translation as a STRING. No code fences, no commentary. Translate EVERY sentence."""


GENDER_INSTR = (
    "A [male] / [female] tag gives the gender of the person that term refers to. "
    "Every {TGT} word that agrees with that person — past-tense verb, adjective, "
    "participle, pronoun — must take that gender, even where {SRC} does not mark it."
)


def render_prompt(src, tgt, glossary, gender=False, instr=False):
    s = TRANSLATE_TMPL.replace("{SRC}", LANG[src]).replace("{TGT}", LANG[tgt])
    if not glossary:
        return s
    s += f"\n\nGLOSSARY — use these {LANG[tgt]} translations consistently wherever the term appears:\n"
    if gender and instr:
        s += GENDER_INSTR.replace("{SRC}", LANG[src]).replace("{TGT}", LANG[tgt]) + "\n"
    for e in glossary:
        line = f"- {e['src']} → {e['tgt']}"
        if gender and e.get("gender"):
            line += "  [male]" if e["gender"] == "m" else "  [female]"
        s += line + "\n"
    return s


def env(path=os.path.join(HERE, "..", ".env")):
    out = {}
    for line in open(path, encoding="utf-8"):
        line = line.strip()
        if line and not line.startswith("#") and "=" in line:
            k, v = line.split("=", 1)
            out[k.strip()] = v.strip()
    return out


class API:
    def __init__(self, cfg, model=None):
        self.key = cfg["OPENROUTER_API_KEY"]
        self.url = cfg.get("OPENROUTER_BASE_URL", "https://openrouter.ai/api/v1") + "/chat/completions"
        self.model = model or cfg.get("MODEL", "google/gemini-3.1-flash-lite")
        self.stats = []

    def chat(self, system, user, retries=5):
        body = json.dumps({
            "model": self.model,
            "messages": [{"role": "system", "content": system},
                         {"role": "user", "content": user}],
            "temperature": 0.3,
            "response_format": {"type": "json_object"},
            "usage": {"include": True},
            "reasoning": {"enabled": False},
        }).encode()
        last = None
        for attempt in range(retries):
            if attempt:
                time.sleep(min(30, 2 ** attempt))
            req = urllib.request.Request(self.url, data=body, headers={
                "Authorization": "Bearer " + self.key, "Content-Type": "application/json"})
            try:
                t0 = time.time()
                with urllib.request.urlopen(req, timeout=180) as r:
                    d = json.loads(r.read())
                u = d.get("usage") or {}
                self.stats.append({"prompt": u.get("prompt_tokens"), "out": u.get("completion_tokens"),
                                   "cost": u.get("cost"), "latency": round(time.time() - t0, 2),
                                   "finish": d["choices"][0].get("finish_reason"),
                                   "cached": (u.get("prompt_tokens_details") or {}).get("cached_tokens")})
                txt = d["choices"][0]["message"]["content"]
                txt = re.sub(r"^```(?:json)?|```$", "", txt.strip(), flags=re.M).strip()
                return json.loads(txt)
            except Exception as e:              # noqa: BLE001 — retry everything
                last = e
        print(f"  ! batch failed: {last}", file=sys.stderr)
        return {}


RENDER_SYS = """You prepare a translation glossary for a book ({SRC} → {TGT}).

You receive a JSON array of CANDIDATE terms mined from the book by frequency, each
{id, term, freq, examples}.

KEEP every proper noun unconditionally — characters, places, organisations, ships,
products. A name has NO single obvious {TGT} rendering: it can be transliterated
several ways, and the reader must meet the same one in every chapter.
KEEP invented or domain terminology and titles.
DROP only ordinary {SRC} words and function words that happen to be capitalised or
frequent ("into", "going", "Real", "Good").

For each KEPT candidate give the {TGT} rendering in its BASE form (nominative
singular for nouns and names) — the form a reader would meet in a dictionary.

Also classify each kept candidate:
  "kind": "person" | "place" | "org" | "thing"
  "gender": for a NAMED INDIVIDUAL ONLY — one specific character — and only when
  the examples or the name itself make it certain: "m" or "f". Omit gender for a
  category of people ("a Catholic", "a Mongol", "the crew"), for a name that
  could belong to more than one character, and whenever the evidence is not
  there. A wrong gender is worse than no gender.

Reply with ONLY a JSON object mapping each kept "id" (exact string) to
{"term":"…","tgt":"…","kind":"…","gender":"m|f"} (gender optional), where "term" is
the candidate's term COPIED EXACTLY from the input. Drop a candidate by omitting its
id. No commentary.

The echoed "term" is checked against the id: an entry whose term does not match is
thrown away, so copy it, do not retype it from memory."""


def do_render(api, cands, src, tgt, batch=50, workers=8):
    sys_p = RENDER_SYS.replace("{SRC}", LANG[src]).replace("{TGT}", LANG[tgt])
    items = [{"id": str(i), "term": c["term"], "freq": c["freq"], "examples": c["examples"][:1]}
             for i, c in enumerate(cands)]
    chunks = [items[i:i + batch] for i in range(0, len(items), batch)]
    out = {}
    with ThreadPoolExecutor(workers) as ex:
        for res in ex.map(lambda ch: api.chat(sys_p, json.dumps(ch, ensure_ascii=False)), chunks):
            out.update(res)
    gloss, dropped = [], 0
    for i, c in enumerate(cands):
        t = out.get(str(i))
        if isinstance(t, str) and t.strip():          # legacy string form
            t = {"tgt": t}
        if not isinstance(t, dict) or not str(t.get("tgt", "")).strip():
            continue
        # Positional id mapping alone is not safe: measured 12/128 entries on
        # Revelation Space glued a rendering to the wrong candidate («aside →
        # Новый Комусо»). Trust the id only when the echoed term matches it.
        echo = str(t.get("term", "")).strip()
        if echo and echo.lower() != c["term"].lower():
            dropped += 1
            continue
        e = {"src": c["term"], "tgt": str(t["tgt"]).strip(), "freq": c["freq"],
             "kind": c["kind"], "entity": t.get("kind")}
        if t.get("gender") in ("m", "f"):
            e["gender"] = t["gender"]
        gloss.append(e)
    gloss.sort(key=lambda e: -e["freq"])
    if dropped:
        print(f"  dropped {dropped} entries whose echoed term did not match its id")
    return gloss


def do_translate(api, probe, gloss, src, tgt, batch=16, workers=16, gender=False, instr=False):
    sys_p = render_prompt(src, tgt, gloss, gender=gender, instr=instr)
    chunks = [probe[i:i + batch] for i in range(0, len(probe), batch)]

    def run(idx_chunk):
        i0, ch = idx_chunk
        items = [{"id": str(j + 1), "src": s["src"]} for j, s in enumerate(ch)]
        res = api.chat(sys_p, json.dumps(items, ensure_ascii=False))
        return [(s, res.get(str(j + 1))) for j, s in enumerate(ch)]

    rows = []
    with ThreadPoolExecutor(workers) as ex:
        for part in ex.map(run, enumerate(chunks)):
            for s, tr in part:
                rows.append({**s, "tr": tr})
    return rows, len(sys_p)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--check", action="store_true")
    ap.add_argument("--render"); ap.add_argument("--translate")
    ap.add_argument("--gloss"); ap.add_argument("--gloss-size", type=int, default=0)
    ap.add_argument("--gender", action="store_true")
    ap.add_argument("--gender-instr", action="store_true")
    ap.add_argument("--src", default="en"); ap.add_argument("--tgt", default="ru")
    ap.add_argument("--model"); ap.add_argument("--out", required=False)
    ap.add_argument("--batch", type=int, default=16)
    ap.add_argument("--limit", type=int, default=0)
    a = ap.parse_args()

    if a.check:
        for name, g in (("nogloss", []), ("gloss2", [{"src": "stack", "tgt": "стэк"},
                                                     {"src": "Ortega", "tgt": "Ортега"}])):
            want = open(os.path.join(ART, f"prompt-go-{name}.txt"), encoding="utf-8").read()
            got = render_prompt("en", "ru", g)
            print(f"{name}: {'IDENTICAL to Go' if got == want else 'DIFFERS'} "
                  f"({len(got)} vs {len(want)} bytes)")
            if got != want:
                import difflib
                print("\n".join(list(difflib.unified_diff(want.splitlines(), got.splitlines()))[:20]))
                sys.exit(1)
        return

    api = API(env(), a.model)
    if a.render:
        cands = json.load(open(a.render))
        if a.limit:
            cands = cands[:a.limit]
        gloss = do_render(api, cands, a.src, a.tgt)
        json.dump(gloss, open(a.out, "w"), ensure_ascii=False, indent=1)
        kept = len(gloss)
        print(f"rendered {kept}/{len(cands)} candidates kept -> {a.out}")
        print(f"  names {sum(1 for e in gloss if e['kind']=='name')}, "
              f"coined {sum(1 for e in gloss if e['kind']=='coined')}")
    elif a.translate:
        probe = json.load(open(a.translate))
        gloss = json.load(open(a.gloss))[:a.gloss_size] if a.gloss else []
        rows, plen = do_translate(api, probe, gloss, a.src, a.tgt, batch=a.batch,
                                  gender=a.gender, instr=a.gender_instr)
        done = sum(1 for r in rows if r["tr"])
        json.dump(rows, open(a.out, "w"), ensure_ascii=False, indent=1)
        print(f"translated {done}/{len(rows)} with {len(gloss)} glossary entries "
              f"(system prompt {plen} chars) -> {a.out}")
    else:
        ap.error("nothing to do")

    st = [s for s in api.stats if s.get("prompt")]
    if st:
        n = len(st)
        print(f"  requests {n}: prompt {sum(s['prompt'] for s in st)/n:.0f} tok avg, "
              f"out {sum(s['out'] for s in st)/n:.0f} tok avg, "
              f"cached {sum((s['cached'] or 0) for s in st)/n:.0f} tok avg, "
              f"cost ${sum((s['cost'] or 0) for s in st):.4f}, "
              f"p50 latency {sorted(s['latency'] for s in st)[n//2]:.1f}s")
        json.dump(api.stats, open((a.out or "probe") + ".stats.json", "w"), indent=1)


if __name__ == "__main__":
    main()
