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
import { ImagePlusIcon, LoaderCircleIcon, SparklesIcon, XIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ModelGroupSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
import { Textarea } from '@/components/ui/textarea'

import { generateImage } from '../../api'
import type {
  GroupOption,
  ImageGenerationResponse,
  ModelOption,
  PlaygroundConfig,
} from '../../types'

type ImageWorkspaceProps = {
  config: PlaygroundConfig
  groups: GroupOption[]
  isModelLoading: boolean
  models: ModelOption[]
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
}

export function ImageWorkspace({
  config,
  groups,
  isModelLoading,
  models,
  onConfigChange,
}: ImageWorkspaceProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [quality, setQuality] = useState('standard')
  const [quantity, setQuantity] = useState(1)
  const [references, setReferences] = useState<string[]>([])
  const [result, setResult] = useState<ImageGenerationResponse | null>(null)
  const [isGenerating, setIsGenerating] = useState(false)

  const handleFiles = async (files: FileList | null) => {
    if (!files?.length) return

    const nextReferences = await Promise.all(
      Array.from(files).map(
        (file) =>
          new Promise<string>((resolve, reject) => {
            const reader = new FileReader()
            reader.onload = () => resolve(String(reader.result))
            reader.onerror = () => reject(reader.error)
            reader.readAsDataURL(file)
          })
      )
    )
    setReferences((current) => [...current, ...nextReferences].slice(0, 4))
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !config.model) return

    setIsGenerating(true)
    try {
      const response = await generateImage({
        model: config.model,
        group: config.group,
        prompt: prompt.trim(),
        n: quantity,
        size,
        quality,
        ...(references.length > 0 ? { images: references } : {}),
      })
      setResult(response)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsGenerating(false)
    }
  }

  const images = result?.data ?? []

  return (
    <div className='grid min-h-0 flex-1 overflow-auto lg:grid-cols-[22rem_minmax(0,1fr)]'>
      <section className='border-border/70 bg-muted/15 flex min-h-0 flex-col border-b p-4 lg:border-r lg:border-b-0'>
        <div className='space-y-4 overflow-y-auto pr-1'>
          <ModelGroupSelector
            disabled={isGenerating || isModelLoading}
            groups={groups}
            models={models}
            onGroupChange={(value) => onConfigChange('group', value)}
            onModelChange={(value) => onConfigChange('model', value)}
            selectedGroup={config.group}
            selectedModel={config.model}
          />

          <div className='space-y-2'>
            <Label htmlFor='image-prompt'>{t('Prompt')}</Label>
            <Textarea
              disabled={isGenerating}
              id='image-prompt'
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={t('Describe the image you want to create')}
              value={prompt}
            />
          </div>

          <div className='grid grid-cols-2 gap-3'>
            <label className='space-y-2 text-sm font-medium'>
              {t('Image size')}
              <NativeSelect
                disabled={isGenerating}
                onChange={(event) => setSize(event.target.value)}
                value={size}
              >
                <NativeSelectOption value='1024x1024'>1:1</NativeSelectOption>
                <NativeSelectOption value='1536x1024'>3:2</NativeSelectOption>
                <NativeSelectOption value='1024x1536'>2:3</NativeSelectOption>
              </NativeSelect>
            </label>
            <label className='space-y-2 text-sm font-medium'>
              {t('Image quality')}
              <NativeSelect
                disabled={isGenerating}
                onChange={(event) => setQuality(event.target.value)}
                value={quality}
              >
                <NativeSelectOption value='standard'>
                  {t('Standard')}
                </NativeSelectOption>
                <NativeSelectOption value='hd'>HD</NativeSelectOption>
              </NativeSelect>
            </label>
            <label className='space-y-2 text-sm font-medium'>
              {t('Quantity')}
              <NativeSelect
                disabled={isGenerating}
                onChange={(event) => setQuantity(Number(event.target.value))}
                value={quantity}
              >
                {[1, 2, 3, 4].map((value) => (
                  <NativeSelectOption key={value} value={value}>
                    {value}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
          </div>

          <div className='space-y-2'>
            <Label>{t('Reference images')}</Label>
            <input
              accept='image/*'
              className='sr-only'
              multiple
              onChange={(event) => {
                void handleFiles(event.target.files)
                event.currentTarget.value = ''
              }}
              ref={inputRef}
              type='file'
            />
            <div className='flex flex-wrap gap-2'>
              {references.map((reference, index) => (
                <div className='relative size-14' key={reference}>
                  <img
                    alt={t('Reference image {{index}}', { index: index + 1 })}
                    className='size-full rounded-md border object-cover'
                    src={reference}
                  />
                  <Button
                    aria-label={t('Remove reference image {{index}}', {
                      index: index + 1,
                    })}
                    className='absolute -top-2 -right-2 bg-background shadow-sm'
                    disabled={isGenerating}
                    onClick={() =>
                      setReferences((current) =>
                        current.filter((_, currentIndex) => currentIndex !== index)
                      )
                    }
                    size='icon-xs'
                    type='button'
                    variant='outline'
                  >
                    <XIcon />
                  </Button>
                </div>
              ))}
              {references.length < 4 && (
                <Button
                  disabled={isGenerating}
                  onClick={() => inputRef.current?.click()}
                  size='icon-lg'
                  type='button'
                  variant='outline'
                >
                  <ImagePlusIcon />
                  <span className='sr-only'>{t('Add reference images')}</span>
                </Button>
              )}
            </div>
          </div>
        </div>
        <Button
          className='mt-4 w-full'
          disabled={isGenerating || !prompt.trim() || !config.model}
          onClick={() => void handleGenerate()}
          size='lg'
          type='button'
        >
          {isGenerating ? (
            <LoaderCircleIcon className='animate-spin' />
          ) : (
            <SparklesIcon />
          )}
          {isGenerating ? t('Generating...') : t('Create image')}
        </Button>
      </section>

      <section className='flex min-h-[24rem] min-w-0 flex-1 items-center justify-center p-4 md:p-6'>
        {images.length > 0 ? (
          <div className='grid w-full max-w-5xl gap-4 sm:grid-cols-2'>
            {images.map((image, index) => (
              <img
                alt={
                  image.revised_prompt ||
                  t('Generated image {{index}}', { index: index + 1 })
                }
                className='aspect-square w-full rounded-lg border bg-muted object-contain'
                key={image.url || image.b64_json || index}
                src={
                  image.url ||
                  (image.b64_json
                    ? `data:image/png;base64,${image.b64_json}`
                    : undefined)
                }
              />
            ))}
          </div>
        ) : (
          <div className='text-muted-foreground flex flex-col items-center gap-3 text-center text-sm'>
            <ImagePlusIcon className='size-9' />
            <span>{t('Generated images appear here')}</span>
          </div>
        )}
      </section>
    </div>
  )
}
