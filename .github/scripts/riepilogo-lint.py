#!/usr/bin/env python3
"""Riepilogo dei finding di golangci-lint per il job informativo `lint-tutto`.

Uso: riepilogo-lint.py <lint.json> <pacchetti.txt> <fmt.txt>
  lint.json      uscita JSON di `golangci-lint run`
  pacchetti.txt  una cartella di pacchetto per riga (`go list -f '{{.Dir}}' ./...`)
  fmt.txt        file che `golangci-lint fmt --diff` cambierebbe, uno per riga

Stampa in Markdown: totale, finding per linter, finding per pacchetto e
l'elenco dei pacchetti gia' puliti (zero finding e formattati), candidati a
.github/lint-bonificati.txt. Non fallisce mai: e' un rapporto, non un controllo.
"""
import collections
import json
import os
import sys


def main() -> int:
    lint_json, pacchetti_txt, fmt_txt = sys.argv[1:4]
    radice = os.getcwd()
    with open(lint_json, encoding="utf-8") as f:
        finding = json.load(f).get("Issues") or []
    with open(pacchetti_txt, encoding="utf-8") as f:
        pacchetti = sorted(os.path.relpath(r.strip(), radice) for r in f if r.strip())
    with open(fmt_txt, encoding="utf-8") as f:
        non_formattati = {os.path.dirname(r.strip()) or "." for r in f if r.strip()}

    per_linter = collections.Counter(i["FromLinter"] for i in finding)
    per_pacchetto = collections.Counter(
        os.path.dirname(i["Pos"]["Filename"]) or "." for i in finding
    )

    print(f"## golangci-lint su tutto il repo: {len(finding)} finding\n")
    print("| Linter | Finding |\n|---|---|")
    for nome, n in per_linter.most_common():
        print(f"| {nome} | {n} |")
    print("\n| Pacchetto | Finding | Formattato |\n|---|---|---|")
    for p in pacchetti:
        if per_pacchetto[p] or p in non_formattati:
            print(f"| `{p}` | {per_pacchetto[p]} | {'no' if p in non_formattati else 'si'} |")
    puliti = [p for p in pacchetti if not per_pacchetto[p] and p not in non_formattati]
    print(f"\n### Pacchetti gia' puliti ({len(puliti)} su {len(pacchetti)})\n")
    print("```")
    for p in puliti:
        print(p)
    print("```")
    return 0


if __name__ == "__main__":
    sys.exit(main())
