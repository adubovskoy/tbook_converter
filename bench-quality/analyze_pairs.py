#!/usr/bin/env python3
"""Analyze blind pairwise verdicts produced by judge agents.

Usage: analyze_pairs.py pairs-dir
Expects in pairs-dir:
  key.json                      from prepare_pairs.py
  verdict-<batch>-o1.json       {id: "X"|"Y"|"tie"} per presentation order
  verdict-<batch>-o2.json

A pair counts as decisive only if both orders agree after de-swapping;
any disagreement or explicit tie -> tie. Sign test on decisive pairs.
"""
import glob, json, math, sys


def binom_two_sided(k, n):
    if n == 0:
        return 1.0
    p = sum(math.comb(n, i) for i in range(0, min(k, n - k) + 1)) / 2 ** n * 2
    return min(1.0, p)


def main():
    d = sys.argv[1]
    key = json.load(open(f'{d}/key.json'))
    la, lb, pairs = key['a_label'], key['b_label'], key['pairs']
    v1, v2 = {}, {}
    for f in glob.glob(f'{d}/verdict-*-o1.json'):
        v1.update(json.load(open(f)))
    for f in glob.glob(f'{d}/verdict-*-o2.json'):
        v2.update(json.load(open(f)))

    def dewrap(sid, v, order):
        # order 1: X = (b if swap else a); order 2 swaps presentation again
        if v not in ('X', 'Y'):
            return 'tie'
        swap = pairs[sid]['swap']
        picked_first = (v == 'X') if order == 1 else (v == 'Y')
        return lb if (picked_first == swap) else la

    wins = {la: 0, lb: 0}
    ties = inconsistent = missing = 0
    for sid in pairs:
        a1, a2 = v1.get(sid), v2.get(sid)
        if a1 is None or a2 is None:
            missing += 1
            continue
        r1, r2 = dewrap(sid, a1, 1), dewrap(sid, a2, 2)
        if r1 == 'tie' or r2 == 'tie':
            ties += 1
        elif r1 != r2:
            inconsistent += 1
        else:
            wins[r1] += 1
    dec = wins[la] + wins[lb]
    print(f'pairs: {len(pairs)}  judged: {len(pairs)-missing}  missing: {missing}')
    print(f'ties: {ties}  order-inconsistent (counted as ties): {inconsistent}')
    print(f'decisive: {dec}')
    for l in (la, lb):
        share = wins[l] / dec if dec else 0
        print(f'  {l}: {wins[l]}  ({share:.1%} of decisive)')
    if dec:
        p = binom_two_sided(min(wins[la], wins[lb]), dec)
        print(f'sign test two-sided p = {p:.4g}')


if __name__ == '__main__':
    main()
