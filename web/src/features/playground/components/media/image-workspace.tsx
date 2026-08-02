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
import {
  ImageIcon,
  ImagePlusIcon,
  LoaderCircleIcon,
  SparklesIcon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import { useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { GroupSelector, ModelSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import {
  Select,
  SelectContent,
  SelectGroup,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select'
import { Textarea } from '@/components/ui/textarea'

import { generateImage, uploadPlaygroundAsset } from '../../api'
import {
  clearImageWorkspaceItems,
  deleteImageWorkspaceItem,
  loadImageWorkspaceItems,
  saveImageWorkspaceResult,
} from '../../lib'
import type {
  ImageWorkspaceItem,
  ImageWorkspaceMetadata,
} from '../../lib/storage/storage'
import type {
  GroupOption,
  ImageGenerationRequest,
  ModelOption,
  PlaygroundConfig,
} from '../../types'
import { ImageGenerationHistory } from './media-generation-history'

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

const maxReferenceImages = 14
const imageSizeOptions = [
  { label: '1:1', value: '1024x1024' },
  { label: '16:9', value: '1792x1024' },
  { label: '9:16', value: '1024x1792' },
  { label: '4:3', value: '1024x768' },
  { label: '3:4', value: '768x1024' },
]
const imageQualityOptions = [
  { label: 'auto', labelKey: 'Auto', value: 'auto' },
  { label: 'fast', labelKey: 'Fast', value: 'fast' },
  { label: 'hd', labelKey: 'HD', value: 'hd' },
  { label: '4k', labelKey: '4K', value: '4k' },
]
const imageQuantityOptions = Array.from(
  { length: 10 },
  (_, index) => index + 1
).map((value) => ({
  label: String(value),
  value: String(value),
}))

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
  const [quality, setQuality] = useState('auto')
  const [quantity, setQuantity] = useState(1)
  const [references, setReferences] = useState<string[]>([])
  const [items, setItems] = useState<ImageWorkspaceItem[]>(
    loadImageWorkspaceItems
  )
  const [isGenerating, setIsGenerating] = useState(false)

  const handleFiles = async (files: FileList | null) => {
    if (!files?.length) return

    try {
      const nextReferences = await Promise.all(
        [...files].map((file) => uploadPlaygroundAsset(file, 'image'))
      )
      setReferences((current) =>
        [...current, ...nextReferences].slice(0, maxReferenceImages)
      )
    } catch {
      toast.error(t('Unable to read image'))
    }
  }

  const generate = async (
    request: ImageGenerationRequest,
    metadata: ImageWorkspaceMetadata
  ) => {
    setIsGenerating(true)
    try {
      const response = await generateImage(request)
      saveImageWorkspaceResult(response, metadata)
      setItems(loadImageWorkspaceItems())
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsGenerating(false)
    }
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !config.model) return

    const aspectRatio =
      imageSizeOptions.find((option) => option.value === size)?.label ?? '1:1'
    await generate(
      {
        model: config.model,
        group: config.group,
        prompt: prompt.trim(),
        n: quantity,
        size,
        ...(quality !== 'auto' ? { quality } : {}),
        ...(references.length > 0 ? { images: references } : {}),
      },
      {
        aspectRatio,
        group: config.group,
        model: config.model,
        n: quantity,
        prompt: prompt.trim(),
        qualityPreset: quality,
        referenceImages: references,
        size,
      }
    )
  }

  const handleRegenerate = (id: string) => {
    const item = items.find((current) => current.id === id)
    if (!item) return

    const itemSize =
      item.size ??
      imageSizeOptions.find((option) => option.label === item.aspectRatio)
        ?.value ??
      '1024x1024'
    onConfigChange('group', item.group)
    onConfigChange('model', item.model)
    setPrompt(item.prompt)
    setSize(itemSize)
    setQuality(item.qualityPreset)
    setQuantity(item.n)
    setReferences(item.referenceImages ?? [])
    void generate(
      {
        model: item.model,
        group: item.group,
        prompt: item.prompt,
        n: item.n,
        size: itemSize,
        ...(item.qualityPreset !== 'auto'
          ? { quality: item.qualityPreset }
          : {}),
        ...(item.referenceImages?.length
          ? { images: item.referenceImages }
          : {}),
      },
      {
        aspectRatio: item.aspectRatio,
        group: item.group,
        model: item.model,
        n: item.n,
        prompt: item.prompt,
        qualityPreset: item.qualityPreset,
        referenceImages: item.referenceImages,
        size: itemSize,
      }
    )
  }

  const handleDelete = (id: string) => {
    deleteImageWorkspaceItem(id)
    setItems((current) => current.filter((item) => item.id !== id))
  }

  const handleClearAll = () => {
    clearImageWorkspaceItems()
    setItems([])
  }

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <header className='border-border/70 shrink-0 border-b px-4 py-3 md:px-6'>
        <div className='mx-auto flex w-full max-w-7xl items-center justify-between gap-3'>
          <div className='flex items-center gap-2 text-sm font-semibold'>
            <ImageIcon className='size-4' />
            {t('Image')}
          </div>
        </div>
      </header>
      <div className='flex-1 overflow-y-auto px-4 py-4 md:px-6'>
        <div className='mx-auto grid w-full max-w-7xl gap-4 lg:grid-cols-[minmax(23rem,28rem)_minmax(0,1fr)] xl:grid-cols-[minmax(24rem,30rem)_minmax(0,1fr)]'>
          <section className='bg-background grid h-fit gap-4 rounded-lg border p-4'>
            <div className='flex items-center gap-2'>
              <GroupSelector
                disabled={isGenerating || isModelLoading}
                groups={groups}
                onGroupChange={(value) => onConfigChange('group', value)}
                selectedGroup={config.group}
              />
              <ModelSelector
                disabled={isGenerating || isModelLoading}
                models={models}
                onModelChange={(value) => onConfigChange('model', value)}
                selectedModel={config.model}
              />
            </div>

            <div className='grid gap-1.5'>
              <Label className='text-xs' htmlFor='image-prompt'>
                {t('Creative description')}
              </Label>
              <Textarea
                className='min-h-36 resize-none'
                disabled={isGenerating}
                id='image-prompt'
                maxLength={5000}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={t('Describe the image you want to create')}
                value={prompt}
              />
            </div>

            <div className='grid grid-cols-2 gap-3 md:grid-cols-3'>
              <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                {t('Aspect ratio')}
                <Select
                  disabled={isGenerating}
                  items={imageSizeOptions}
                  onValueChange={(value) => {
                    if (value) setSize(value)
                  }}
                  value={size}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {imageSizeOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
              <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                {t('Clarity')}
                <Select
                  disabled={isGenerating}
                  items={imageQualityOptions}
                  onValueChange={(value) => {
                    if (value) setQuality(value)
                  }}
                  value={quality}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {imageQualityOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {t(option.labelKey)}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
              <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                {t('Quantity')}
                <Select
                  disabled={isGenerating}
                  items={imageQuantityOptions}
                  onValueChange={(value) => setQuantity(Number(value))}
                  value={String(quantity)}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {imageQuantityOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
            </div>

            <div className='bg-muted/20 rounded-md border p-3'>
              <div className='flex items-center justify-between gap-3'>
                <Label>
                  <ImagePlusIcon className='size-4' />
                  {t('Reference images')}
                  <span className='text-muted-foreground bg-muted rounded-full px-1.5 py-0.5 text-xs'>
                    {references.length}/{maxReferenceImages}
                  </span>
                </Label>
                <input
                  accept='image/png,image/jpeg,image/webp'
                  className='sr-only'
                  multiple
                  onChange={(event) => {
                    void handleFiles(event.target.files)
                    event.currentTarget.value = ''
                  }}
                  ref={inputRef}
                  type='file'
                />
                <Button
                  disabled={
                    isGenerating || references.length >= maxReferenceImages
                  }
                  onClick={() => inputRef.current?.click()}
                  size='sm'
                  type='button'
                  variant='outline'
                >
                  <UploadIcon />
                  {t('Upload')}
                </Button>
              </div>
              {references.length > 0 ? (
                <div className='mt-3 flex flex-wrap gap-2'>
                  {references.map((reference, index) => (
                    <div className='relative size-14' key={reference}>
                      <img
                        alt={t('Reference image {{index}}', {
                          index: index + 1,
                        })}
                        className='size-full rounded-md border object-cover'
                        src={reference}
                      />
                      <Button
                        aria-label={t('Remove reference image {{index}}', {
                          index: index + 1,
                        })}
                        className='bg-background absolute -top-2 -right-2 shadow-sm'
                        disabled={isGenerating}
                        onClick={() =>
                          setReferences((current) =>
                            current.filter(
                              (_, currentIndex) => currentIndex !== index
                            )
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
                </div>
              ) : (
                <p className='border-border bg-muted/40 text-muted-foreground mt-3 rounded-md border px-3 py-2 text-xs leading-relaxed'>
                  {t('Optional reference media')}
                </p>
              )}
            </div>

            {!isModelLoading && models.length === 0 && (
              <p className='border-border bg-muted/40 text-muted-foreground rounded-md border px-3 py-2 text-xs leading-relaxed'>
                {t(
                  'No models are available for this group. Choose another group or ask an administrator to enable a compatible model.'
                )}
              </p>
            )}
            <Button
              className='w-full'
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

          <ImageGenerationHistory
            isRegenerating={isGenerating}
            items={items}
            onClear={handleClearAll}
            onDelete={handleDelete}
            onRegenerate={handleRegenerate}
          />
        </div>
      </div>
    </div>
  )
}
