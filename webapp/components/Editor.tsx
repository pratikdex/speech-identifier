'use client';

import { useEffect, useRef } from 'react';
import { useEditor, EditorContent, Extension, type JSONContent } from '@tiptap/react';
import StarterKit from '@tiptap/starter-kit';
import { TableKit } from '@tiptap/extension-table';
import type { Utterance } from '@/lib/dictation';

// Confidence (0..1) of the recognized speech, shown as a colored circle in the left margin.
const Confidence = Extension.create({
  name: 'confidence',
  addGlobalAttributes: () => [
    {
      types: ['paragraph'],
      attributes: {
        confidence: {
          default: null,
          keepOnSplit: false,
          parseHTML: (el) => (el.dataset.confidence ? Number(el.dataset.confidence) / 100 : null),
          renderHTML: ({ confidence }) => {
            if (typeof confidence !== 'number') return {};
            const level = confidence >= 0.85 ? 'high' : confidence >= 0.6 ? 'mid' : 'low';
            return { 'data-confidence': Math.round(confidence * 100), 'data-level': level };
          },
        },
      },
    },
  ],
});

const para = (text: string, confidence?: number): JSONContent => ({
  type: 'paragraph',
  attrs: { confidence: confidence ?? null },
  content: text ? [{ type: 'text', text }] : [],
});
const cell = (p: JSONContent): JSONContent => ({ type: 'tableCell', content: [p] });

// Editor appends each final utterance to the end of the document, so the user can
// edit freely while dictation keeps flowing in. When translating, each utterance is a
// table row: what was said on the left, its translation on the right.
export function Editor({ lines, translate }: { lines: Utterance[]; translate: boolean }) {
  const inserted = useRef(0);
  const editor = useEditor({
    extensions: [StarterKit, TableKit, Confidence],
    immediatelyRender: false,
    editorProps: { attributes: { class: 'outline-none min-h-40', 'aria-label': 'Transcript' } },
  });

  useEffect(() => {
    if (!editor) return;
    const finals = lines.filter((l) => l.final);
    if (finals.length < inserted.current) inserted.current = 0; // new session cleared lines
    for (const l of finals.slice(inserted.current)) {
      const text = l.text.trim();
      if (!text) continue;
      // Nodes, not a string: strings are parsed as HTML.
      const said = para(text, l.confidence);
      const row: JSONContent = { type: 'tableRow', content: [cell(said), cell(para(l.translation?.trim() ?? ''))] };
      const block: JSONContent = translate ? { type: 'table', content: [row] } : said;

      const { doc } = editor.state;
      // StarterKit keeps an empty paragraph after a trailing table; look past it.
      let last = doc.lastChild;
      let end = doc.content.size;
      if (last?.type.name === 'paragraph' && !last.content.size && doc.childCount > 1) {
        end -= last.nodeSize;
        last = doc.child(doc.childCount - 2);
      }
      if (translate && last?.type.name === 'table') editor.commands.insertContentAt(end - 1, row);
      else if (editor.isEmpty) editor.commands.setContent(block);
      else editor.commands.insertContentAt(doc.content.size, block);
    }
    inserted.current = finals.length;
  }, [editor, lines, translate]);

  return <EditorContent editor={editor} />;
}
