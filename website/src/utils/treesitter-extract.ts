// src/utils/treesitter-extract.ts
import Parser, { Query } from "tree-sitter";
import Go from "tree-sitter-go";
import Rust from "tree-sitter-rust";
import TS from "tree-sitter-typescript";

const tsLang = TS.typescript;
const rustLang = Rust;
const goLang = Go;

export type Lang = "ts" | "rust" | "go";

export interface ExtractResult {
  name: string;
  snippet: string; // empty if not found
  found: boolean;
}

/** Build and cache parsers + queries once. */
const parserByLang: Record<Lang, Parser> = {
  ts: new Parser(),
  rust: new Parser(),
  go: new Parser(),
};

parserByLang.ts.setLanguage(tsLang);
parserByLang.rust.setLanguage(rustLang);
parserByLang.go.setLanguage(goLang);

/** Tree-sitter query patterns to capture declarations + their names. */
const qTs = new Query(
  tsLang,
  `
  (interface_declaration name: (type_identifier) @name) @decl
  (type_alias_declaration name: (type_identifier) @name) @decl
  (enum_declaration name: (identifier) @name) @decl
  (class_declaration name: (type_identifier) @name) @decl
`,
);

const qRust = new Query(
  rustLang,
  `
  (struct_item name: (type_identifier) @name) @decl
  (enum_item   name: (type_identifier) @name) @decl
  (type_item   name: (type_identifier) @name) @decl
  (trait_item  name: (type_identifier) @name) @decl
`,
);

const qGo = new Query(
  goLang,
  `
  (type_spec name: (type_identifier) @name) @decl
`,
);

const queries: Record<Lang, Query> = { ts: qTs, rust: qRust, go: qGo };

/** Extend a node slice upward to include contiguous doc comments. */
function sliceWithLeadingComments(
  src: string,
  node: Parser.SyntaxNode,
): string {
  let start = node.startIndex;
  let prev: Parser.SyntaxNode | null = node.previousSibling;
  while (prev && prev.type === "comment") {
    start = prev.startIndex;
    prev = prev.previousSibling;
  }
  return src.slice(start, node.endIndex);
}

/** Extract declarations by name for a given language. */
export function extractNamed(
  src: string,
  names: string[],
  lang: Lang,
): ExtractResult[] {
  const parser = parserByLang[lang];
  const query = queries[lang];
  const tree = parser.parse(src);

  const matches = query.matches(tree.rootNode);

  interface Hit {
    name: string;
    decl: Parser.SyntaxNode;
  }
  const hits: Hit[] = [];
  for (const m of matches) {
    let nameText: string | null = null;
    let declNode: Parser.SyntaxNode | null = null;
    for (const c of m.captures) {
      if (c.name === "name") nameText = c.node.text;
      if (c.name === "decl") declNode = c.node;
    }
    if (nameText && declNode) hits.push({ name: nameText, decl: declNode });
  }

  const byName = new Map<string, Parser.SyntaxNode[]>();
  for (const h of hits) {
    (byName.get(h.name) ?? byName.set(h.name, []).get(h.name)!).push(h.decl);
  }

  return names.map((n) => {
    const decls = byName.get(n);
    if (!decls?.length) {
      return {
        name: n,
        snippet: `// Could not find declaration for "${n}"`,
        found: false,
      };
    }
    const parts = decls.map((d) => {
      let text = sliceWithLeadingComments(src, d).trim();
      // 👇 Fix: Go type_spec lacks the leading "type"
      if (lang === "go" && !/^\s*type\s/.test(text)) {
        text = `type ${text}`;
      }
      return text;
    });
    return { name: n, snippet: parts.join("\n\n"), found: true };
  });
}

/** Convenience: return a single combined string with separators. */
export function extractCombined(
  src: string,
  names: string[],
  lang: Lang,
): string {
  const results = extractNamed(src, names, lang);
  return results
    .filter((r) => r.found)
    .map((r) => r.snippet)
    .join("\n\n");
}
