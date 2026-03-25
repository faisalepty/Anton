---
name: web-search
---

## Before your first search — rewrite the query

Never search with the raw user query. First reason about it:

- What is the user actually asking for?
- What type of source would have this answer?
- What is the simplest, most direct query that finds it?

Examples:
```
User asks: "S&P 500 latest"
Bad:  "S&P 500 index value March 2026 latest performance data"
Good: "S&P 500 today"  +  "S&P 500 March 2026 close"

User asks: "palantir latest news"
Bad:  "Palantir Technologies news updates March 2026 latest developments"
Good: "Palantir news 2026"  +  "Palantir earnings 2026"
```

Start SHORT and BROAD — 2-4 words per query. Long specific queries return fewer results.
Generate 2 queries max. Run both, read results, then decide if more is needed.

---

## How to evaluate what you get back

After every search, ask in order:

1. Does this contain a specific fact/number/date with a named source?
   YES → that is your answer. STOP. Report it with the URL.

2. Is this a projection or estimate rather than a real value?
   YES → not the answer. Run one more search with a different angle.

3. Is this from a low-quality source (SEO blog, unnamed site)?
   YES → use only if nothing better exists.

4. Have I run 3 searches and still don't have the answer?
   YES → report what was found, note what could not be confirmed. STOP.

---

## Tool usage

tavily_search  — primary. Use time_range: week or month for recent topics.
                 Use include_domains for authoritative sources:
                 Financial: ["reuters.com", "bloomberg.com", "marketwatch.com", "finance.yahoo.com"]
                 News:      ["reuters.com", "bbc.com", "apnews.com"]

tavily_extract — use when a result URL looks important but content is thin.

fetch_page     — last resort if tavily_extract fails.

---

## Hard limits

- Maximum 3 search rounds
- Found a specific answer with any credible source → STOP immediately
- Never search to confirm what you already found
- Never fabricate sources, numbers, or URLs