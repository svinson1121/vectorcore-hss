import React from 'react'
import CodeMirror from '@uiw/react-codemirror'
import { xml } from '@codemirror/lang-xml'
import { oneDark } from '@codemirror/theme-one-dark'
import { EditorView, keymap } from '@codemirror/view'
import { indentWithTab } from '@codemirror/commands'

const IFC_EDITOR_THEME = EditorView.theme({
  '&': {
    fontSize: '12px',
    border: '1px solid var(--border)',
    borderRadius: 'var(--radius-sm)',
    overflow: 'hidden',
  },
  '.cm-editor': {
    height: '100%',
    backgroundColor: 'var(--bg-input)',
  },
  '.cm-scroller': {
    fontFamily: 'var(--font-mono)',
    lineHeight: '1.6',
  },
  '.cm-gutters': {
    backgroundColor: 'var(--bg-elevated)',
    color: 'var(--text-muted)',
    borderRight: '1px solid var(--border-subtle)',
  },
  '.cm-activeLine': {
    backgroundColor: 'rgba(88, 166, 255, 0.08)',
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'rgba(88, 166, 255, 0.1)',
  },
  '.cm-content': {
    paddingTop: '8px',
    paddingBottom: '8px',
  },
  '.cm-line': {
    paddingLeft: '10px',
    paddingRight: '10px',
  },
  '.cm-focused': {
    outline: 'none',
  },
  '.cm-selectionBackground, ::selection': {
    backgroundColor: 'rgba(88, 166, 255, 0.28)',
  },
})

export default function IFCCodeEditor({ value, onChange, validation, rows = 16 }) {
  return (
    <div>
      <div style={{ minHeight: rows * 22 + 40 }}>
        <CodeMirror
          value={value}
          height={`${rows * 22 + 40}px`}
          theme={oneDark}
          extensions={[
            xml(),
            keymap.of([indentWithTab]),
            IFC_EDITOR_THEME,
            EditorView.lineWrapping,
          ]}
          basicSetup={{
            foldGutter: true,
            highlightActiveLine: true,
            highlightActiveLineGutter: true,
            lineNumbers: true,
            tabSize: 2,
          }}
          onChange={onChange}
        />
      </div>
      <div
        style={{
          marginTop: 8,
          fontSize: '0.75rem',
          color: validation.valid ? 'var(--success)' : 'var(--warning)',
          background: validation.valid ? 'var(--success-bg)' : 'var(--warning-bg)',
          border: `1px solid ${validation.valid ? 'color-mix(in srgb, var(--success) 35%, transparent)' : 'color-mix(in srgb, var(--warning) 35%, transparent)'}`,
          borderRadius: 'var(--radius-sm)',
          padding: '8px 10px',
        }}
      >
        {validation.valid ? 'Syntax check passed.' : `Syntax check failed. ${validation.message}`}
      </div>
    </div>
  )
}
