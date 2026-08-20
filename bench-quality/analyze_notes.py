#!/usr/bin/env python3
"""Defect profile from judge notes written by judge_pairs.py.

Usage: analyze_notes.py pairs-dir

For every decisive judgement (either presentation order) the judge named the
losing side's worst defect class; this aggregates those classes per arm, i.e.
"what the judges punished this arm for". Counts are judgements, not pairs, so a
pair judged consistently in both orders contributes twice.
"""
import collections, glob, json, sys


def main():
    d = sys.argv[1]
    key = json.load(open(f"{d}/key.json"))
    la, lb, pairs = key["a_label"], key["b_label"], key["pairs"]
    loser = collections.defaultdict(collections.Counter)
    for order in (1, 2):
        for f in glob.glob(f"{d}/notes-*-o{order}.json"):
            for sid, g in json.load(open(f)).items():
                w = g.get("w")
                if w not in ("X", "Y"):
                    continue
                swap = pairs[sid]["swap"]
                picked_first = (w == "X") if order == 1 else (w == "Y")
                winner = lb if (picked_first == swap) else la
                loser[la if winner == lb else lb][g.get("d") or "unspecified"] += 1
    classes = sorted({c for arm in loser.values() for c in arm},
                     key=lambda c: -(loser[la][c] + loser[lb][c]))
    print(f"{'defect class':22} {la:>12} {lb:>12}")
    for c in classes:
        print(f"{c:22} {loser[la][c]:>12} {loser[lb][c]:>12}")
    print(f"{'TOTAL':22} {sum(loser[la].values()):>12} {sum(loser[lb].values()):>12}")


if __name__ == "__main__":
    main()
