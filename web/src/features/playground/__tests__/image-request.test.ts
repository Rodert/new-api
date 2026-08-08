import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import { createImageEditFormData } from '../lib/image-request.ts'

describe('createImageEditFormData', () => {
  test('preserves multiple image files and their original extensions', () => {
    const first = new File(['first'], 'reference-one.jpg', {
      type: 'image/jpeg',
    })
    const second = new File(['second'], 'reference-two.png', {
      type: 'image/png',
    })

    const formData = createImageEditFormData(
      {
        model: 'gpt-image-1',
        group: 'default',
        prompt: 'edit both images',
        n: 2,
        size: '1024x1024',
        quality: 'hd',
      },
      [first, second]
    )

    assert.equal(formData.get('model'), 'gpt-image-1')
    assert.equal(formData.get('group'), 'default')
    assert.equal(formData.get('prompt'), 'edit both images')
    assert.equal(formData.get('n'), '2')
    assert.equal(formData.get('size'), '1024x1024')
    assert.equal(formData.get('quality'), 'hd')
    assert.deepEqual(
      formData.getAll('image[]').map((entry) => (entry as File).name),
      ['reference-one.jpg', 'reference-two.png']
    )
  })
})
