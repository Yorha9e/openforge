import { DiffEditor } from '@monaco-editor/react';

export function DiffPanel() {
  const original = '// Original code\nfunction hello() {\n  console.log("hello");\n}';
  const modified = '// Modified code\nfunction hello() {\n  console.log("hello world");\n}';

  return (
    <DiffEditor
      height="100%"
      original={original}
      modified={modified}
      language="typescript"
      theme="vs-dark"
      options={{ readOnly: true, minimap: { enabled: false } }}
    />
  );
}
