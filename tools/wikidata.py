"""Fetch the CC0 facts the knowledge base leans on from Wikidata.

Wikidata supplies what the FCI standards do not: a stable English name for each
breed, the country of origin, a Wikipedia article and a Commons picture -- all
CC0.  Three queries, three CSVs, consumed by rebuild_spine.py and heights.py.
"""
import sys, time, urllib.parse, urllib.request

UA = "ribice-kb-build/0.1 (+https://codecrane.hr/ribice; building a free identification quiz)"
ENDPOINT = "https://query.wikidata.org/sparql"

QUERIES = {
 # every dog breed, with aliases, for matching FCI names that carry no FCI number
 "wd-all.csv": """
SELECT ?breed ?breedLabel ?alias ?originLabel ?img ?article WHERE {
  ?breed wdt:P31 wd:Q39367 .
  OPTIONAL { ?breed skos:altLabel ?alias FILTER(LANG(?alias)="en") }
  OPTIONAL { ?breed wdt:P495 ?origin }
  OPTIONAL { ?breed wdt:P18 ?img }
  OPTIONAL { ?article schema:about ?breed ; schema:isPartOf <https://en.wikipedia.org/> }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}""",
 # breeds carrying an FCI breed number: P528 qualified by P972 = the FCI (Q38603)
 "wd-spine.csv": """
SELECT ?breed ?breedLabel ?fci ?originLabel ?img ?article ?commons WHERE {
  ?breed wdt:P31 wd:Q39367 ; p:P528 ?st .
  ?st ps:P528 ?fci ; pq:P972 wd:Q38603 .
  OPTIONAL { ?breed wdt:P495 ?origin }
  OPTIONAL { ?breed wdt:P18 ?img }
  OPTIONAL { ?breed wdt:P373 ?commons }
  OPTIONAL { ?article schema:about ?breed ; schema:isPartOf <https://en.wikipedia.org/> }
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}""",
 # height at withers, used to cross-check the figures parsed from the standards
 # and to fill in where a standard gives none
 "wd-height.csv": """
SELECT ?breed ?h ?unitLabel WHERE {
  ?breed wdt:P31 wd:Q39367 ; p:P2048 ?st .
  ?st psv:P2048 ?v . ?v wikibase:quantityAmount ?h ; wikibase:quantityUnit ?unit .
  SERVICE wikibase:label { bd:serviceParam wikibase:language "en". }
}""",
}

def main():
    for name, query in QUERIES.items():
        url = ENDPOINT + "?" + urllib.parse.urlencode({"query": query})
        req = urllib.request.Request(url, headers={"User-Agent": UA,
                                                   "Accept": "text/csv"})
        with urllib.request.urlopen(req, timeout=120) as r:
            data = r.read()
        open(name, "wb").write(data)
        print(f"  {name}: {data.count(chr(10)) - 1} rows", file=sys.stderr)
        time.sleep(1)

main()
