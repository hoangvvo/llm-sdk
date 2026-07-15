import { readFile } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { Language, Parser, Query } from "web-tree-sitter";

const VIRTUAL_MODULE_ID = "virtual:type-snippets";
const RESOLVED_MODULE_ID = `\0${VIRTUAL_MODULE_ID}`;

const sourceFiles = {
  sdk: {
    ts: new URL("../../sdk-js/src/types.ts", import.meta.url),
    rust: new URL("../../sdk-rust/src/types.rs", import.meta.url),
    go: new URL("../../sdk-go/types.go", import.meta.url),
  },
  agentTypes: {
    ts: new URL("../../agent-js/src/types.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/types.rs", import.meta.url),
    go: new URL("../../agent-go/types.go", import.meta.url),
  },
  agentParams: {
    ts: new URL("../../agent-js/src/params.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/params.rs", import.meta.url),
    go: new URL("../../agent-go/params.go", import.meta.url),
  },
  agentTool: {
    ts: new URL("../../agent-js/src/tool.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/tool.rs", import.meta.url),
    go: new URL("../../agent-go/tool.go", import.meta.url),
  },
  agentToolkit: {
    ts: new URL("../../agent-js/src/toolkit.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/toolkit.rs", import.meta.url),
    go: new URL("../../agent-go/toolkit.go", import.meta.url),
  },
  agentMcp: {
    ts: new URL("../../agent-js/src/mcp/types.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/mcp/types.rs", import.meta.url),
    go: new URL("../../agent-go/mcp/types.go", import.meta.url),
  },
  agentInstruction: {
    ts: new URL("../../agent-js/src/instruction.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/instruction.rs", import.meta.url),
    go: new URL("../../agent-go/instruction.go", import.meta.url),
  },
  agentRun: {
    ts: new URL("../../agent-js/src/run.ts", import.meta.url),
    rust: new URL("../../agent-rust/src/run.rs", import.meta.url),
    go: new URL("../../agent-go/run.go", import.meta.url),
  },
};

const sourcePaths = new Set(
  Object.values(sourceFiles).flatMap((files) =>
    Object.values(files).map(fileURLToPath),
  ),
);

const querySources = {
  ts: `
    (interface_declaration name: (type_identifier) @name) @decl
    (type_alias_declaration name: (type_identifier) @name) @decl
    (enum_declaration name: (identifier) @name) @decl
    (class_declaration name: (type_identifier) @name) @decl
  `,
  rust: `
    (struct_item name: (type_identifier) @name) @decl
    (enum_item   name: (type_identifier) @name) @decl
    (type_item   name: (type_identifier) @name) @decl
    (trait_item  name: (type_identifier) @name) @decl
  `,
  go: `(type_spec name: (type_identifier) @name) @decl`,
};

let toolsPromise;

async function createTools() {
  await Parser.init();

  const languages = {
    ts: await Language.load(
      fileURLToPath(
        import.meta
          .resolve("tree-sitter-typescript/tree-sitter-typescript.wasm"),
      ),
    ),
    rust: await Language.load(
      fileURLToPath(
        import.meta.resolve("tree-sitter-rust/tree-sitter-rust.wasm"),
      ),
    ),
    go: await Language.load(
      fileURLToPath(import.meta.resolve("tree-sitter-go/tree-sitter-go.wasm")),
    ),
  };

  return Object.fromEntries(
    Object.entries(languages).map(([lang, language]) => {
      const parser = new Parser();
      parser.setLanguage(language);
      return [lang, { parser, query: new Query(language, querySources[lang]) }];
    }),
  );
}

function sliceWithLeadingComments(source, node) {
  let start = node.startIndex;
  let previous = node.previousSibling;
  while (previous?.type === "comment") {
    start = previous.startIndex;
    previous = previous.previousSibling;
  }
  return source.slice(start, node.endIndex);
}

function extractDeclarations(source, lang, { parser, query }) {
  const tree = parser.parse(source);
  if (!tree) throw new Error(`Tree-sitter failed to parse ${lang} source`);

  const snippets = new Map();
  for (const match of query.matches(tree.rootNode)) {
    const name = match.captures.find((capture) => capture.name === "name")?.node
      .text;
    const declaration = match.captures.find(
      (capture) => capture.name === "decl",
    )?.node;
    if (!name || !declaration) continue;

    let snippet = sliceWithLeadingComments(source, declaration).trim();
    if (lang === "go" && !/^\s*type\s/.test(snippet)) {
      snippet = `type ${snippet}`;
    }

    const declarations = snippets.get(name) ?? [];
    declarations.push(snippet);
    snippets.set(name, declarations);
  }

  tree.delete();
  return Object.fromEntries(
    [...snippets].map(([name, declarations]) => [
      name,
      declarations.join("\n\n"),
    ]),
  );
}

async function buildCatalog() {
  const tools = await (toolsPromise ??= createTools());
  const catalog = {};

  for (const [group, files] of Object.entries(sourceFiles)) {
    catalog[group] = {};
    for (const [lang, file] of Object.entries(files)) {
      const source = await readFile(file, "utf8");
      catalog[group][lang] = extractDeclarations(source, lang, tools[lang]);
    }
  }

  return catalog;
}

export function typeSnippets() {
  let catalogPromise;

  return {
    name: "type-snippets",
    resolveId(id) {
      if (id === VIRTUAL_MODULE_ID) return RESOLVED_MODULE_ID;
    },
    async load(id) {
      if (id !== RESOLVED_MODULE_ID) return;
      for (const sourcePath of sourcePaths) this.addWatchFile(sourcePath);
      const catalog = await (catalogPromise ??= buildCatalog());
      return `export default ${JSON.stringify(catalog)};`;
    },
    watchChange(id) {
      if (sourcePaths.has(id)) catalogPromise = undefined;
    },
    hotUpdate({ file, modules }) {
      if (!sourcePaths.has(file)) return;

      catalogPromise = undefined;
      const virtualModule =
        this.environment.moduleGraph.getModuleById(RESOLVED_MODULE_ID);
      if (!virtualModule || modules.includes(virtualModule)) return modules;
      return [...modules, virtualModule];
    },
  };
}
