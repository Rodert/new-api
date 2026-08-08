/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { after, test } from 'node:test'

import { Window } from 'happy-dom'

const domWindow = new Window()
const domGlobals = [
  'window',
  'document',
  'navigator',
  'HTMLElement',
  'HTMLAnchorElement',
  'Node',
  'Element',
  'Event',
  'CustomEvent',
  'MutationObserver',
  'ResizeObserver',
  'requestAnimationFrame',
  'cancelAnimationFrame',
  'getComputedStyle',
] as const

for (const key of domGlobals) {
  Object.defineProperty(globalThis, key, {
    configurable: true,
    value: domWindow[key],
  })
}

const { act } = await import('react')
const { createRoot } = await import('react-dom/client')
const i18next = (await import('i18next')).default
const { initReactI18next } = await import('react-i18next')
await i18next.use(initReactI18next).init({
  lng: 'en',
  resources: {
    en: {
      translation: {
        'Open in new tab': 'Open in new tab',
      },
    },
  },
})
const { ImageGenerationHistory } = await import('../media-generation-history')
const reactTestGlobals = globalThis as typeof globalThis & {
  IS_REACT_ACT_ENVIRONMENT?: boolean
}
reactTestGlobals.IS_REACT_ACT_ENVIRONMENT = true

after(() => {
  domWindow.close()
})

test('opens a generated image URL in a new tab from the enlarge action', async () => {
  const source = 'https://file.example.com/generated/image.png'
  const container = document.createElement('div')
  document.body.append(container)
  const root = createRoot(container)

  await act(async () => {
    root.render(
      <ImageGenerationHistory
        isRegenerating={false}
        items={[
          {
            id: 'image-1',
            prompt: 'Draw a landscape',
            model: 'image-model',
            group: 'default',
            aspectRatio: '1:1',
            qualityPreset: 'standard',
            n: 1,
            createdAt: 1,
            status: 'completed',
            size: '1024x1024',
            data: [{ url: source }],
          },
        ]}
        onClear={() => undefined}
        onDelete={() => undefined}
        onRegenerate={() => undefined}
      />
    )
  })

  const enlargeLink = container.querySelector<HTMLAnchorElement>(
    'a[aria-label="Open in new tab"]'
  )
  assert.ok(enlargeLink)
  assert.equal(enlargeLink.href, source)
  assert.equal(enlargeLink.target, '_blank')
  assert.equal(enlargeLink.rel, 'noreferrer')

  await act(async () => root.unmount())
  container.remove()
})
