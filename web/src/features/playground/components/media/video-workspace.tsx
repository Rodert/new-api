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
  AudioLinesIcon,
  FileVideoIcon,
  FilmIcon,
  ImagePlusIcon,
  InfoIcon,
  type LucideIcon,
  LoaderCircleIcon,
  SparklesIcon,
  Trash2Icon,
  UploadIcon,
  XIcon,
} from 'lucide-react'
import {
  type Dispatch,
  type RefObject,
  type SetStateAction,
  useEffect,
  useRef,
  useState,
} from 'react'
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

import { createVideo, getVideoTask } from '../../api'
import type {
  GroupOption,
  ModelOption,
  PlaygroundConfig,
  VideoTaskResponse,
} from '../../types'

type VideoWorkspaceProps = {
  config: PlaygroundConfig
  groups: GroupOption[]
  isModelLoading: boolean
  models: ModelOption[]
  onConfigChange: <K extends keyof PlaygroundConfig>(
    key: K,
    value: PlaygroundConfig[K]
  ) => void
}

type LocalMedia = {
  name: string
  url: string
}

const completedStatuses = new Set(['succeeded', 'success', 'completed'])
const failedStatuses = new Set(['failed', 'error', 'cancelled', 'canceled'])
const maxReferenceImages = 4
const maxReferenceVideos = 3
const maxReferenceAudio = 1
const videoRatioOptions = [
  { label: '1:1', value: '1:1' },
  { label: '16:9', value: '16:9' },
  { label: '9:16', value: '9:16' },
]
const videoDurationOptions = [5, 10, 15].map((value) => ({
  label: `${value}s`,
  value: String(value),
}))
const videoQualityOptions = [
  { label: 'standard', labelKey: 'Standard', value: 'standard' },
  { label: 'fast', labelKey: 'Fast', value: 'fast' },
  { label: 'hd', labelKey: 'HD', value: 'hd' },
]
const videoQuantityOptions = Array.from(
  { length: 10 },
  (_, index) => index + 1
).map((value) => ({
  label: String(value),
  value: String(value),
}))

export function VideoWorkspace({
  config,
  groups,
  isModelLoading,
  models,
  onConfigChange,
}: VideoWorkspaceProps) {
  const { t } = useTranslation()
  const imageInputRef = useRef<HTMLInputElement>(null)
  const videoInputRef = useRef<HTMLInputElement>(null)
  const audioInputRef = useRef<HTMLInputElement>(null)
  const [prompt, setPrompt] = useState('')
  const [referenceImages, setReferenceImages] = useState<LocalMedia[]>([])
  const [referenceVideos, setReferenceVideos] = useState<LocalMedia[]>([])
  const [referenceAudio, setReferenceAudio] = useState<LocalMedia[]>([])
  const [ratio, setRatio] = useState('16:9')
  const [duration, setDuration] = useState(15)
  const [quality, setQuality] = useState('hd')
  const [quantity, setQuantity] = useState(1)
  const [task, setTask] = useState<VideoTaskResponse | null>(null)
  const [isSubmitting, setIsSubmitting] = useState(false)

  const taskId = task?.task_id || task?.id
  const status = task?.status?.toLowerCase()
  const isCompleted = Boolean(status && completedStatuses.has(status))
  const isFailed = Boolean(status && failedStatuses.has(status))

  useEffect(() => {
    if (!taskId || isCompleted || isFailed) return

    const timer = window.setInterval(() => {
      void getVideoTask(taskId)
        .then(setTask)
        .catch((error) => {
          toast.error(
            error instanceof Error ? error.message : t('Request failed')
          )
        })
    }, 3000)

    return () => window.clearInterval(timer)
  }, [isCompleted, isFailed, t, taskId])

  const handleFiles = async (
    files: FileList | null,
    maximum: number,
    setMedia: Dispatch<SetStateAction<LocalMedia[]>>
  ) => {
    if (!files?.length) return

    try {
      const media = await Promise.all(
        [...files].map(
          (file) =>
            new Promise<LocalMedia>((resolve, reject) => {
              const reader = new FileReader()
              reader.addEventListener(
                'load',
                () => resolve({ name: file.name, url: String(reader.result) }),
                { once: true }
              )
              reader.addEventListener('error', () => reject(reader.error), {
                once: true,
              })
              reader.readAsDataURL(file)
            })
        )
      )
      setMedia((current) => [...current, ...media].slice(0, maximum))
    } catch {
      toast.error(t('Unable to read reference media'))
    }
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !config.model) return

    let dimensions = [1280, 720]
    if (ratio === '9:16') {
      dimensions = [720, 1280]
    } else if (ratio === '1:1') {
      dimensions = [1024, 1024]
    }
    setIsSubmitting(true)
    setTask(null)
    try {
      const response = await createVideo({
        model: config.model,
        group: config.group,
        prompt: prompt.trim(),
        duration,
        width: dimensions[0],
        height: dimensions[1],
        n: quantity,
        quality,
        ...(referenceImages[0] ? { image: referenceImages[0].url } : {}),
      })
      setTask(response)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleClear = () => {
    setPrompt('')
    setReferenceImages([])
    setReferenceVideos([])
    setReferenceAudio([])
    setTask(null)
  }

  const videoURL =
    task?.url || (isCompleted && taskId ? `/v1/videos/${taskId}/content` : null)
  const hasMedia =
    referenceImages.length > 0 ||
    referenceVideos.length > 0 ||
    referenceAudio.length > 0
  let resultDescription = t('Generated video appears here')
  if (isFailed) {
    resultDescription = task?.error?.message || t('Video generation failed')
  } else if (taskId) {
    resultDescription = t('Waiting for video...')
  }

  const renderMediaCard = (
    title: string,
    helper: string,
    icon: LucideIcon,
    inputRef: RefObject<HTMLInputElement | null>,
    accept: string,
    media: LocalMedia[],
    maximum: number,
    setMedia: Dispatch<SetStateAction<LocalMedia[]>>,
    showThumbnails = false
  ) => {
    const Icon = icon
    return (
      <div className='bg-muted/20 rounded-md border p-3'>
        <div className='flex items-center justify-between gap-3'>
          <Label>
            <Icon className='size-4' />
            {title}
            <span className='text-muted-foreground bg-muted rounded-full px-1.5 py-0.5 text-xs'>
              {media.length}/{maximum}
            </span>
          </Label>
          <input
            accept={accept}
            className='sr-only'
            onChange={(event) => {
              void handleFiles(event.target.files, maximum, setMedia)
              event.currentTarget.value = ''
            }}
            ref={inputRef}
            type='file'
          />
          <Button
            disabled={isSubmitting || media.length >= maximum}
            onClick={() => inputRef.current?.click()}
            size='sm'
            type='button'
            variant='outline'
          >
            <UploadIcon />
            {t('Upload')}
          </Button>
        </div>
        {media.length > 0 ? (
          <div className='mt-3 flex flex-wrap gap-2'>
            {media.map((item, index) => (
              <div
                className={
                  showThumbnails
                    ? 'relative size-14'
                    : 'border-border bg-muted relative flex h-9 max-w-full items-center gap-2 rounded-md border py-1 pr-8 pl-2 text-xs'
                }
                key={item.url}
              >
                {showThumbnails ? (
                  <img
                    alt={t('Reference image {{index}}', { index: index + 1 })}
                    className='size-full rounded-md border object-cover'
                    src={item.url}
                  />
                ) : (
                  <span className='truncate'>{item.name}</span>
                )}
                <Button
                  aria-label={t('Remove reference media {{index}}', {
                    index: index + 1,
                  })}
                  className='bg-background absolute -top-2 -right-2 shadow-sm'
                  disabled={isSubmitting}
                  onClick={() =>
                    setMedia((current) =>
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
            {helper}
          </p>
        )}
      </div>
    )
  }

  return (
    <div className='flex min-h-0 flex-1 flex-col overflow-hidden'>
      <header className='border-border/70 shrink-0 border-b px-4 py-3 md:px-6'>
        <div className='mx-auto flex w-full max-w-7xl items-center justify-between gap-3'>
          <div className='flex items-center gap-2 text-sm font-semibold'>
            <FilmIcon className='size-4' />
            {t('Video')}
          </div>
          <Button
            disabled={isSubmitting || (!prompt && !task && !hasMedia)}
            onClick={handleClear}
            size='sm'
            type='button'
            variant='ghost'
          >
            <Trash2Icon />
            {t('Clear')}
          </Button>
        </div>
      </header>
      <div className='flex-1 overflow-y-auto px-4 py-4 md:px-6'>
        <div className='mx-auto grid w-full max-w-7xl gap-4 lg:grid-cols-[minmax(23rem,28rem)_minmax(0,1fr)] xl:grid-cols-[minmax(24rem,30rem)_minmax(0,1fr)]'>
          <section className='bg-background grid h-fit gap-4 rounded-lg border p-4'>
            <div className='flex items-center gap-2'>
              <GroupSelector
                disabled={isSubmitting || isModelLoading}
                groups={groups}
                onGroupChange={(value) => onConfigChange('group', value)}
                selectedGroup={config.group}
              />
              <ModelSelector
                disabled={isSubmitting || isModelLoading}
                models={models}
                onModelChange={(value) => onConfigChange('model', value)}
                selectedModel={config.model}
              />
            </div>
            <div className='grid gap-1.5'>
              <Label className='text-xs' htmlFor='video-prompt'>
                {t('Creative description')}
              </Label>
              <Textarea
                className='min-h-36 resize-none'
                disabled={isSubmitting}
                id='video-prompt'
                maxLength={5000}
                onChange={(event) => setPrompt(event.target.value)}
                placeholder={t('Describe the video you want to create')}
                value={prompt}
              />
              <div className='text-muted-foreground flex items-center justify-between text-xs'>
                <span>{t('Video prompt limit')}</span>
                <span>{prompt.length}/5000</span>
              </div>
            </div>
            <div className='grid grid-cols-2 gap-3 md:grid-cols-4'>
              <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                {t('Aspect ratio')}
                <Select
                  disabled={isSubmitting}
                  items={videoRatioOptions}
                  onValueChange={(value) => {
                    if (value) setRatio(value)
                  }}
                  value={ratio}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {videoRatioOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
              <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                {t('Duration')}
                <Select
                  disabled={isSubmitting}
                  items={videoDurationOptions}
                  onValueChange={(value) => setDuration(Number(value))}
                  value={String(duration)}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {videoDurationOptions.map((option) => (
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
                  disabled={isSubmitting}
                  items={videoQualityOptions}
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
                      {videoQualityOptions.map((option) => (
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
                  disabled={isSubmitting}
                  items={videoQuantityOptions}
                  onValueChange={(value) => setQuantity(Number(value))}
                  value={String(quantity)}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {videoQuantityOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
            </div>

            {renderMediaCard(
              t('Reference images'),
              t('Optional reference media'),
              ImagePlusIcon,
              imageInputRef,
              'image/png,image/jpeg,image/webp',
              referenceImages,
              maxReferenceImages,
              setReferenceImages,
              true
            )}
            {renderMediaCard(
              t('Reference videos'),
              t('3-10 seconds'),
              FileVideoIcon,
              videoInputRef,
              'video/mp4,video/quicktime,video/webm',
              referenceVideos,
              maxReferenceVideos,
              setReferenceVideos
            )}
            {renderMediaCard(
              t('Reference audio'),
              t('2-15 seconds, up to 15 MB'),
              AudioLinesIcon,
              audioInputRef,
              'audio/mpeg,audio/mp4,audio/wav,audio/aac,audio/ogg',
              referenceAudio,
              maxReferenceAudio,
              setReferenceAudio
            )}

            {!isModelLoading && models.length === 0 && (
              <p className='border-border bg-muted/40 text-muted-foreground rounded-md border px-3 py-2 text-xs leading-relaxed'>
                {t(
                  'No models are available for this group. Choose another group or ask an administrator to enable a compatible model.'
                )}
              </p>
            )}

            {(referenceVideos.length > 0 || referenceAudio.length > 0) && (
              <div className='flex gap-2 rounded-lg border border-amber-500/30 bg-amber-500/10 p-3 text-xs'>
                <InfoIcon className='mt-0.5 size-4 shrink-0' />
                <p>
                  {t(
                    'Video and audio references are kept in this page only and will be sent after backend support is added.'
                  )}
                </p>
              </div>
            )}
            <Button
              className='w-full'
              disabled={isSubmitting || !prompt.trim() || !config.model}
              onClick={() => void handleGenerate()}
              size='lg'
              type='button'
            >
              {isSubmitting ? (
                <LoaderCircleIcon className='animate-spin' />
              ) : (
                <SparklesIcon />
              )}
              {isSubmitting ? t('Generating...') : t('Create video')}
            </Button>
          </section>

          <section className='border-border/70 flex min-h-80 min-w-0 items-center justify-center rounded-lg border border-dashed p-8 text-center'>
            {videoURL ? (
              <video
                className='max-h-[70vh] max-w-full rounded-lg border bg-black'
                controls
                src={videoURL}
              />
            ) : (
              <div className='text-muted-foreground flex flex-col items-center gap-2 text-center text-sm'>
                {taskId && !isFailed ? (
                  <LoaderCircleIcon className='size-9 animate-spin' />
                ) : (
                  <FilmIcon className='size-9' />
                )}
                <strong className='text-foreground'>
                  {isFailed ? t('Video generation failed') : t('No videos yet')}
                </strong>
                <span>{resultDescription}</span>
              </div>
            )}
          </section>
        </div>
      </div>
    </div>
  )
}
