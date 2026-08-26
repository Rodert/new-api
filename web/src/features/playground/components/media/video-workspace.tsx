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
  type LucideIcon,
  LoaderCircleIcon,
  SparklesIcon,
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
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group'

import { createVideo, getVideoTask, uploadPlaygroundAsset } from '../../api'
import {
  clearVideoWorkspaceTasks,
  deleteVideoWorkspaceTask,
  loadVideoWorkspaceTasks,
  saveVideoWorkspaceTask,
} from '../../lib'
import type {
  VideoWorkspaceMetadata,
  VideoWorkspaceTask,
} from '../../lib/storage/storage'
import type { GroupOption, ModelOption, PlaygroundConfig } from '../../types'
import { VideoGenerationHistory } from './media-generation-history'

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

type GrokVideoImageMode = 'first_frame' | 'reference'

const grokImagineVideo15Models = new Set(['grok-imagine-video-1.5'])

const completedStatuses = new Set(['succeeded', 'success', 'completed'])
const failedStatuses = new Set(['failed', 'error', 'cancelled', 'canceled'])
const defaultMediaLimits = {
  images: 4,
  videos: 3,
  audios: 1,
}
const videoRatioOptions = [
  { label: '1:1', value: '1:1' },
  { label: '16:9', value: '16:9' },
  { label: '9:16', value: '9:16' },
]
const defaultVideoDurationOptions = [5, 10, 15].map((value) => ({
  label: `${value}s`,
  value: String(value),
}))
const grokVideo15DurationOptions = [4, 6, 8, 10, 12, 15].map((value) => ({
  label: `${value}s`,
  value: String(value),
}))
const seedance25DurationOptions = Array.from({ length: 27 }, (_, index) => {
  const value = index + 4
  return { label: `${value}s`, value: String(value) }
})
const klingVideoDurationOptions = Array.from({ length: 13 }, (_, index) => {
  const value = index + 3
  return { label: `${value}s`, value: String(value) }
})
const minimaxH3DurationOptions = Array.from({ length: 11 }, (_, index) => {
  const value = index + 5
  return { label: `${value}s`, value: String(value) }
})
const videoResolutionOptionsByModel: Record<string, string[]> = {
  'drama-video-v2': ['480p', '720p', '1080p'],
  'drama-video-v2-fast': ['480p', '720p'],
  'grok-imagine-video': ['480p', '720p', '1080p'],
  'grok-imagine-video-1.5-preview': ['480p', '720p', '1080p'],
  'grok-video-1.5': ['480p', '720p', '1080p'],
  'grok-imagine-video-1.5': ['480p', '720p', '1080p'],
  'kling-video-v3': ['720p', '1080p', '4k'],
  'kling-video-v3-omni': ['720p', '1080p', '4k'],
  'kling-video-v3-turbo': ['720p', '1080p'],
  'video-ds-2.5': ['720p'],
  'video-ds-2.5-480': ['480p'],
  'seedance2.5': ['480p', '720p'],
  'minimax-h3': ['2k'],
}
const mediaLimitsByModel: Record<string, typeof defaultMediaLimits> = {
  'seedance2.5': {
    images: 30,
    videos: 10,
    audios: 10,
  },
}
const emptyResolutionOptions: string[] = []
function getVideoDimensions(ratio: string): [number, number] {
  if (ratio === '9:16') return [720, 1280]
  if (ratio === '1:1') return [1024, 1024]
  return [1280, 720]
}

function toLocalMedia(urls: string[] | undefined): LocalMedia[] {
  return (urls ?? []).map((url) => ({
    name: url.split('/').pop() || url,
    url,
  }))
}

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
  const [grokVideoImageMode, setGrokVideoImageMode] =
    useState<GrokVideoImageMode>('first_frame')
  const [ratio, setRatio] = useState('16:9')
  const [duration, setDuration] = useState(15)
  const [resolution, setResolution] = useState('')
  const [tasks, setTasks] = useState<VideoWorkspaceTask[]>(
    loadVideoWorkspaceTasks
  )
  const [isSubmitting, setIsSubmitting] = useState(false)
  let durationOptions = defaultVideoDurationOptions
  if (config.model === 'seedance2.5') {
    durationOptions = seedance25DurationOptions
  } else if (config.model === 'minimax-h3') {
    durationOptions = minimaxH3DurationOptions
  } else if (
    config.model === 'kling-video-v3' ||
    config.model === 'kling-video-v3-omni' ||
    config.model === 'kling-video-v3-turbo'
  ) {
    durationOptions = klingVideoDurationOptions
  } else if (
    config.model === 'grok-video-1.5' ||
    grokImagineVideo15Models.has(config.model)
  ) {
    durationOptions = grokVideo15DurationOptions
  }
  const mediaLimits = mediaLimitsByModel[config.model] ?? defaultMediaLimits
  const isGrokImagineVideo15 = grokImagineVideo15Models.has(config.model)
  const isGrokVideo15ReferenceMode =
    isGrokImagineVideo15 && grokVideoImageMode === 'reference'
  const imageLimit = isGrokVideo15ReferenceMode
    ? 7
    : isGrokImagineVideo15
      ? 1
      : mediaLimits.images
  const baseResolutionOptions =
    videoResolutionOptionsByModel[config.model] ?? emptyResolutionOptions
  const resolutionOptions = isGrokVideo15ReferenceMode
    ? baseResolutionOptions.filter((value) => value !== '1080p')
    : baseResolutionOptions
  const selectedResolution = resolutionOptions.includes(resolution)
    ? resolution
    : (resolutionOptions[0] ?? '')

  useEffect(() => {
    if (!durationOptions.some((option) => Number(option.value) === duration)) {
      setDuration(Number(durationOptions[0].value))
    }
  }, [config.model, duration, durationOptions])

  useEffect(() => {
    if (!resolutionOptions.length) {
      setResolution('')
      return
    }
    if (!resolutionOptions.includes(resolution)) {
      setResolution(resolutionOptions[0])
    }
  }, [resolution, resolutionOptions])

  useEffect(() => {
    const pendingTaskIDs = tasks
      .filter((task) => {
        const status = task.status?.toLowerCase()
        return (
          !status ||
          (!completedStatuses.has(status) && !failedStatuses.has(status))
        )
      })
      .map((task) => task.taskId)
    if (!pendingTaskIDs.length) return

    let cancelled = false

    const timer = window.setInterval(() => {
      void Promise.all(pendingTaskIDs.map((taskID) => getVideoTask(taskID)))
        .then((responses) => {
          if (cancelled) return
          responses.forEach((response) => saveVideoWorkspaceTask(response))
          setTasks(loadVideoWorkspaceTasks())
        })
        .catch((error) => {
          toast.error(
            error instanceof Error ? error.message : t('Request failed')
          )
        })
    }, 3000)

    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [t, tasks])

  const handleFiles = async (
    files: FileList | null,
    kind: 'audio' | 'image' | 'video',
    maximum: number,
    setMedia: Dispatch<SetStateAction<LocalMedia[]>>
  ) => {
    if (!files?.length) return

    try {
      const media = await Promise.all(
        [...files].map(async (file) => ({
          name: file.name,
          url: await uploadPlaygroundAsset(file, kind),
        }))
      )
      setMedia((current) => [...current, ...media].slice(0, maximum))
    } catch {
      toast.error(t('Unable to read reference media'))
    }
  }

  const createVideoTask = async (metadata: VideoWorkspaceMetadata) => {
    const [width, height] = getVideoDimensions(metadata.aspectRatio)
    setIsSubmitting(true)
    try {
      const isGrokImagineVideo15Task = grokImagineVideo15Models.has(
        metadata.model
      )
      const response = await createVideo({
        model: metadata.model,
        group: metadata.group,
        prompt: metadata.prompt,
        seconds: metadata.seconds,
        aspect_ratio: metadata.aspectRatio,
        duration: Number(metadata.seconds),
        width,
        height,
        ...(metadata.resolution ? { resolution: metadata.resolution } : {}),
        ...(isGrokImagineVideo15Task && metadata.referenceImages?.length
          ? metadata.grokVideoImageMode === 'reference'
            ? { reference_images: metadata.referenceImages }
            : { image: metadata.referenceImages[0] }
          : metadata.referenceImages?.length
            ? { images: metadata.referenceImages }
            : {}),
        ...(metadata.referenceVideos?.length
          ? { videos: metadata.referenceVideos }
          : {}),
        ...(metadata.referenceAudio?.length
          ? { audios: metadata.referenceAudio }
          : {}),
      })
      saveVideoWorkspaceTask(response, metadata)
      setTasks(loadVideoWorkspaceTasks())
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !config.model) return

    await createVideoTask({
      aspectRatio: ratio,
      group: config.group,
      model: config.model,
      prompt: prompt.trim(),
      grokVideoImageMode: isGrokImagineVideo15
        ? grokVideoImageMode
        : undefined,
      resolution: selectedResolution,
      referenceAudio: referenceAudio.map((media) => media.url),
      referenceImages: referenceImages.map((media) => media.url),
      referenceVideos: referenceVideos.map((media) => media.url),
      seconds: String(duration),
    })
  }

  const handleRegenerate = (taskID: string) => {
    const task = tasks.find((current) => current.taskId === taskID)
    if (!task) return

    onConfigChange('group', task.group)
    onConfigChange('model', task.model)
    setPrompt(task.prompt)
    setRatio(task.aspectRatio)
    setDuration(Number(task.seconds))
    setResolution(task.resolution ?? '')
    if (grokImagineVideo15Models.has(task.model)) {
      setGrokVideoImageMode(task.grokVideoImageMode ?? 'first_frame')
    }
    setReferenceImages(toLocalMedia(task.referenceImages))
    setReferenceVideos(toLocalMedia(task.referenceVideos))
    setReferenceAudio(toLocalMedia(task.referenceAudio))
    void createVideoTask({
      aspectRatio: task.aspectRatio,
      group: task.group,
      model: task.model,
      prompt: task.prompt,
      grokVideoImageMode:
        grokImagineVideo15Models.has(task.model)
          ? (task.grokVideoImageMode ?? 'first_frame')
          : undefined,
      resolution: task.resolution,
      referenceAudio: task.referenceAudio,
      referenceImages: task.referenceImages,
      referenceVideos: task.referenceVideos,
      seconds: task.seconds,
    })
  }

  const handleDelete = (taskID: string) => {
    deleteVideoWorkspaceTask(taskID)
    setTasks((current) => current.filter((task) => task.taskId !== taskID))
  }

  const handleClearAll = () => {
    clearVideoWorkspaceTasks()
    setTasks([])
  }

  const renderMediaCard = (
    title: string,
    helper: string,
    icon: LucideIcon,
    inputRef: RefObject<HTMLInputElement | null>,
    accept: string,
    kind: 'audio' | 'image' | 'video',
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
              void handleFiles(event.target.files, kind, maximum, setMedia)
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
                  items={durationOptions}
                  onValueChange={(value) => setDuration(Number(value))}
                  value={String(duration)}
                >
                  <SelectTrigger className='w-full'>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectGroup>
                      {durationOptions.map((option) => (
                        <SelectItem key={option.value} value={option.value}>
                          {option.label}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  </SelectContent>
                </Select>
              </label>
              {resolutionOptions.length ? (
                <label className='text-muted-foreground grid gap-1.5 text-xs font-medium'>
                  {t('Resolution')}
                  <Select
                    disabled={isSubmitting || resolutionOptions.length === 1}
                    items={resolutionOptions.map((value) => ({
                      label: value,
                      value,
                    }))}
                    onValueChange={(value) => {
                      if (value) setResolution(value)
                    }}
                    value={selectedResolution}
                  >
                    <SelectTrigger className='w-full'>
                      <SelectValue />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectGroup>
                        {resolutionOptions.map((value) => (
                          <SelectItem key={value} value={value}>
                            {value}
                          </SelectItem>
                        ))}
                      </SelectGroup>
                    </SelectContent>
                  </Select>
                </label>
              ) : null}
            </div>

            {isGrokImagineVideo15 ? (
              <div className='grid gap-1.5'>
                <Label className='text-xs'>{t('Image mode')}</Label>
                <ToggleGroup
                  disabled={isSubmitting}
                  onValueChange={(value) => {
                    const mode = value.find(
                      (item) => item !== grokVideoImageMode
                    ) as GrokVideoImageMode | undefined
                    if (!mode) return
                    setGrokVideoImageMode(mode)
                    if (mode === 'first_frame') {
                      setReferenceImages((current) => current.slice(0, 1))
                    }
                  }}
                  size='sm'
                  value={[grokVideoImageMode]}
                  variant='outline'
                >
                  <ToggleGroupItem value='first_frame'>
                    {t('First frame')}
                  </ToggleGroupItem>
                  <ToggleGroupItem value='reference'>
                    {t('Reference images')}
                  </ToggleGroupItem>
                </ToggleGroup>
              </div>
            ) : null}

            {renderMediaCard(
              isGrokImagineVideo15 && grokVideoImageMode === 'first_frame'
                ? t('First frame image')
                : t('Reference images'),
              t('Optional reference media'),
              ImagePlusIcon,
              imageInputRef,
              'image/png,image/jpeg,image/webp',
              'image',
              referenceImages,
              imageLimit,
              setReferenceImages,
              true
            )}
            {renderMediaCard(
              t('Reference videos'),
              t('3-10 seconds'),
              FileVideoIcon,
              videoInputRef,
              'video/mp4,video/quicktime,video/webm',
              'video',
              referenceVideos,
              mediaLimits.videos,
              setReferenceVideos
            )}
            {renderMediaCard(
              t('Reference audio'),
              t('2-15 seconds, up to 15 MB'),
              AudioLinesIcon,
              audioInputRef,
              'audio/mpeg,audio/mp4,audio/wav,audio/aac,audio/ogg',
              'audio',
              referenceAudio,
              mediaLimits.audios,
              setReferenceAudio
            )}

            {!isModelLoading && models.length === 0 && (
              <p className='border-border bg-muted/40 text-muted-foreground rounded-md border px-3 py-2 text-xs leading-relaxed'>
                {t(
                  'No models are available for this group. Choose another group or ask an administrator to enable a compatible model.'
                )}
              </p>
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

          <VideoGenerationHistory
            isRegenerating={isSubmitting}
            onClear={handleClearAll}
            onDelete={handleDelete}
            onRegenerate={handleRegenerate}
            tasks={tasks}
          />
        </div>
      </div>
    </div>
  )
}
