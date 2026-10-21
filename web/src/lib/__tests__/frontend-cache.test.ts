/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published by
the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import assert from 'node:assert/strict'
import { afterEach, test } from 'node:test'

import { Window } from 'happy-dom'

import { initializeFrontendCache } from '../frontend-cache'

const originalWindow = Object.getOwnPropertyDescriptor(globalThis, 'window')

afterEach(() => {
  if (originalWindow) {
    Object.defineProperty(globalThis, 'window', originalWindow)
    return
  }
  Reflect.deleteProperty(globalThis, 'window')
})

test('preserves playground media history during a frontend cache reset', () => {
  const domWindow = new Window()
  Object.defineProperty(globalThis, 'window', {
    configurable: true,
    value: domWindow,
  })
  const workspace = JSON.stringify({ image: ['image'], video: ['video'] })
  domWindow.localStorage.setItem('playground_workspace_v1', workspace)
  domWindow.localStorage.setItem('disposable-ui-cache', 'remove me')

  initializeFrontendCache()

  assert.equal(
    domWindow.localStorage.getItem('playground_workspace_v1'),
    workspace
  )
  assert.equal(domWindow.localStorage.getItem('disposable-ui-cache'), null)
  domWindow.close()
})
