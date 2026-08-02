import assert from 'node:assert/strict'
import { describe, test } from 'node:test'

import {
  CHANNEL_TYPE_JIMENG_ZZ_VIDEO,
  CHANNEL_TYPE_OPTIONS,
  MODEL_FETCHABLE_TYPES,
} from '../../constants'
import { CHANNEL_FORM_DEFAULT_VALUES, channelFormSchema } from '../channel-form'
import { getChannelTypeConfig } from '../channel-type-config'
import { getChannelTypeIcon, getKeyPromptForType } from '../channel-utils'

function jimengZZVideoForm(baseUrl: string) {
  return {
    ...CHANNEL_FORM_DEFAULT_VALUES,
    name: 'JimengZZVideo upstream',
    type: CHANNEL_TYPE_JIMENG_ZZ_VIDEO,
    base_url: baseUrl,
    key: 'test-key',
    models: 'video-ds-2.0-fast',
  }
}

describe('JimengZZVideo channel', () => {
  test('registers video-channel selection metadata', () => {
    const option = CHANNEL_TYPE_OPTIONS.find(
      (item) => item.value === CHANNEL_TYPE_JIMENG_ZZ_VIDEO
    )

    assert.deepEqual(option, {
      value: CHANNEL_TYPE_JIMENG_ZZ_VIDEO,
      label: 'JimengZZVideo',
    })
    assert.equal(MODEL_FETCHABLE_TYPES.has(CHANNEL_TYPE_JIMENG_ZZ_VIDEO), false)
    assert.equal(getChannelTypeIcon(CHANNEL_TYPE_JIMENG_ZZ_VIDEO), 'Jimeng')
    assert.equal(
      getKeyPromptForType(CHANNEL_TYPE_JIMENG_ZZ_VIDEO),
      'Enter API key for this channel'
    )
    assert.equal(getChannelTypeConfig(CHANNEL_TYPE_JIMENG_ZZ_VIDEO).icon, 'Jimeng')
  })

  test('requires a non-blank Base URL', () => {
    assert.equal(channelFormSchema.safeParse(jimengZZVideoForm('  ')).success, false)
    assert.equal(
      channelFormSchema.safeParse(
        jimengZZVideoForm('https://upstream.example')
      ).success,
      true
    )
  })
})
