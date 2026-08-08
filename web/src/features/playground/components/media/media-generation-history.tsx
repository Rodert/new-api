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
  DownloadIcon,
  ImageIcon,
  LoaderCircleIcon,
  Maximize2Icon,
  PlayIcon,
  RefreshCwIcon,
  Trash2Icon,
} from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { Button } from '@/components/ui/button'
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from '@/components/ui/tooltip'

import { getVideoContent } from '../../api'
import type {
  ImageWorkspaceItem,
  VideoWorkspaceTask,
} from '../../lib/storage/storage'
import { getVideoURL } from '../../lib/video-task'

const completedStatuses = new Set(['succeeded', 'success', 'completed'])
const failedStatuses = new Set(['failed', 'error', 'cancelled', 'canceled'])

type HistoryActionsProps = {
  onDelete: () => void
  onRegenerate: () => void
  isRegenerating: boolean
}

type ImageGenerationHistoryProps = {
  items: ImageWorkspaceItem[]
  onClear: () => void
  onDelete: (id: string) => void
  onRegenerate: (id: string) => void
  isRegenerating: boolean
}

type VideoGenerationHistoryProps = {
  tasks: VideoWorkspaceTask[]
  onClear: () => void
  onDelete: (taskId: string) => void
  onRegenerate: (taskId: string) => void
  isRegenerating: boolean
}

type ImageGenerationCardProps = HistoryActionsProps & {
  item: ImageWorkspaceItem
}

type VideoGenerationCardProps = HistoryActionsProps & {
  task: VideoWorkspaceTask
}

function MetadataValue(props: { label: string; value: string | number }) {
  return (
    <div className='min-w-0'>
      <dt className='text-muted-foreground text-[11px] font-medium'>
        {props.label}
      </dt>
      <dd className='mt-0.5 truncate text-xs font-medium'>{props.value}</dd>
    </div>
  )
}

function RecordActions(props: HistoryActionsProps) {
  const { t } = useTranslation()

  return (
    <div className='flex items-center gap-1'>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label={t('Regenerate')}
              disabled={props.isRegenerating}
              onClick={props.onRegenerate}
              size='icon-xs'
              type='button'
              variant='ghost'
            >
              <RefreshCwIcon />
            </Button>
          }
        />
        <TooltipContent>{t('Regenerate')}</TooltipContent>
      </Tooltip>
      <Tooltip>
        <TooltipTrigger
          render={
            <Button
              aria-label={t('Delete')}
              disabled={props.isRegenerating}
              onClick={props.onDelete}
              size='icon-xs'
              type='button'
              variant='ghost'
            >
              <Trash2Icon />
            </Button>
          }
        />
        <TooltipContent>{t('Delete')}</TooltipContent>
      </Tooltip>
    </div>
  )
}

function ImageGenerationCard(props: ImageGenerationCardProps) {
  const { t } = useTranslation()

  return (
    <article className='bg-background overflow-hidden rounded-lg border'>
      <div className='flex items-start justify-between gap-3 border-b px-3 py-2.5'>
        <div className='min-w-0'>
          <p className='text-xs font-semibold'>{t('Completed')}</p>
          <p className='text-muted-foreground mt-0.5 text-xs'>
            {t('Created')} {new Date(props.item.createdAt).toLocaleString()}
          </p>
        </div>
        <RecordActions
          isRegenerating={props.isRegenerating}
          onDelete={props.onDelete}
          onRegenerate={props.onRegenerate}
        />
      </div>
      <div className='grid gap-3 p-3'>
        <p className='line-clamp-3 text-sm leading-relaxed'>
          {props.item.prompt}
        </p>
        <dl className='grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-3'>
          <MetadataValue label={t('Model')} value={props.item.model} />
          <MetadataValue label={t('Group')} value={props.item.group} />
          <MetadataValue
            label={t('Image size')}
            value={props.item.size ?? props.item.aspectRatio}
          />
          <MetadataValue
            label={t('Aspect ratio')}
            value={props.item.aspectRatio}
          />
          <MetadataValue
            label={t('Clarity')}
            value={props.item.qualityPreset}
          />
          <MetadataValue label={t('Quantity')} value={props.item.n} />
        </dl>
        {props.item.referenceImages?.length ? (
          <p className='text-muted-foreground text-xs'>
            {t('Reference images')}: {props.item.referenceImages.length}
          </p>
        ) : null}
        <div className='grid gap-2 sm:grid-cols-2'>
          {props.item.data.map((image, index) => {
            const source =
              image.url ||
              (image.b64_json
                ? `data:image/png;base64,${image.b64_json}`
                : undefined)
            if (!source) return null

            return (
              <figure
                className='bg-muted relative overflow-hidden rounded-md border'
                key={image.url || image.b64_json || index}
              >
                <img
                  alt={
                    image.revised_prompt ||
                    t('Generated image {{index}}', { index: index + 1 })
                  }
                  className='aspect-square w-full object-contain'
                  src={source}
                />
                <div className='absolute right-2 bottom-2 flex gap-1'>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          aria-label={t('Open in new tab')}
                          className='bg-background/90 shadow-sm'
                          render={
                            <a href={source} rel='noreferrer' target='_blank' />
                          }
                          size='icon-xs'
                          variant='outline'
                        >
                          <Maximize2Icon />
                        </Button>
                      }
                    />
                    <TooltipContent>{t('Open in new tab')}</TooltipContent>
                  </Tooltip>
                  <Tooltip>
                    <TooltipTrigger
                      render={
                        <Button
                          aria-label={t('Download')}
                          className='bg-background/90 shadow-sm'
                          render={<a download href={source} />}
                          size='icon-xs'
                          variant='outline'
                        >
                          <DownloadIcon />
                        </Button>
                      }
                    />
                    <TooltipContent>{t('Download')}</TooltipContent>
                  </Tooltip>
                </div>
              </figure>
            )
          })}
        </div>
      </div>
    </article>
  )
}

function VideoGenerationCard(props: VideoGenerationCardProps) {
  const { t } = useTranslation()
  const videoRef = useRef<HTMLVideoElement>(null)
  const [videoURL, setVideoURL] = useState<string | null>(null)
  const status = props.task.status?.toLowerCase()
  const isCompleted = Boolean(status && completedStatuses.has(status))
  const isFailed = Boolean(status && failedStatuses.has(status))
  const contentURL = getVideoURL(props.task)

  useEffect(() => {
    if (!contentURL) {
      setVideoURL(null)
      return
    }

    let objectURL: string | null = null
    let cancelled = false

    void getVideoContent(contentURL)
      .then((videoContent) => {
        if (cancelled) return
        objectURL = URL.createObjectURL(videoContent)
        setVideoURL(objectURL)
      })
      .catch((error) => {
        if (cancelled) return
        toast.error(
          error instanceof Error ? error.message : t('Request failed')
        )
      })

    return () => {
      cancelled = true
      if (objectURL) URL.revokeObjectURL(objectURL)
    }
  }, [contentURL, t])

  let statusLabel = t('Generating...')
  if (isCompleted) statusLabel = t('Completed')
  if (isFailed) statusLabel = t('Failed')
  if (status === 'queued') statusLabel = t('Queued')

  return (
    <article className='bg-background overflow-hidden rounded-lg border'>
      <div className='flex items-start justify-between gap-3 border-b px-3 py-2.5'>
        <div className='min-w-0'>
          <p className='text-xs font-semibold'>{statusLabel}</p>
          <p className='text-muted-foreground mt-0.5 truncate font-mono text-[11px]'>
            {t('Task ID')}: {props.task.taskId}
          </p>
        </div>
        <RecordActions
          isRegenerating={props.isRegenerating}
          onDelete={props.onDelete}
          onRegenerate={props.onRegenerate}
        />
      </div>
      <div className='grid gap-3 p-3'>
        <p className='line-clamp-3 text-sm leading-relaxed'>
          {props.task.prompt}
        </p>
        <dl className='grid grid-cols-2 gap-x-3 gap-y-2 sm:grid-cols-3'>
          <MetadataValue label={t('Model')} value={props.task.model} />
          <MetadataValue label={t('Group')} value={props.task.group} />
          <MetadataValue
            label={t('Aspect ratio')}
            value={props.task.aspectRatio}
          />
          <MetadataValue
            label={t('Duration')}
            value={`${props.task.seconds}s`}
          />
          <MetadataValue
            label={t('Clarity')}
            value={props.task.qualityPreset}
          />
          <MetadataValue label={t('Quantity')} value={props.task.n ?? 1} />
        </dl>
        <div className='text-muted-foreground flex flex-wrap gap-x-3 gap-y-1 text-xs'>
          {props.task.referenceImages?.length ? (
            <span>
              {t('Reference images')}: {props.task.referenceImages.length}
            </span>
          ) : null}
          {props.task.referenceVideos?.length ? (
            <span>
              {t('Reference videos')}: {props.task.referenceVideos.length}
            </span>
          ) : null}
          {props.task.referenceAudio?.length ? (
            <span>
              {t('Reference audio')}: {props.task.referenceAudio.length}
            </span>
          ) : null}
        </div>
        {isFailed ? (
          <p className='text-destructive text-xs'>
            {props.task.error?.message || t('Video generation failed')}
          </p>
        ) : null}
        {isCompleted && videoURL ? (
          <div className='grid gap-2'>
            <video
              className='max-h-96 w-full rounded-md border bg-black'
              controls
              ref={videoRef}
              src={videoURL}
            />
            <div className='flex justify-end gap-1'>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      aria-label={t('Play')}
                      onClick={() => void videoRef.current?.play()}
                      size='icon-xs'
                      type='button'
                      variant='outline'
                    >
                      <PlayIcon />
                    </Button>
                  }
                />
                <TooltipContent>{t('Play')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger
                  render={
                    <Button
                      aria-label={t('Download')}
                      render={<a download href={videoURL} />}
                      size='icon-xs'
                      variant='outline'
                    >
                      <DownloadIcon />
                    </Button>
                  }
                />
                <TooltipContent>{t('Download')}</TooltipContent>
              </Tooltip>
            </div>
          </div>
        ) : null}
        {!isCompleted && !isFailed ? (
          <div className='text-muted-foreground flex items-center gap-2 text-xs'>
            <LoaderCircleIcon className='size-3.5 animate-spin' />
            <span>{props.task.progress ?? 0}%</span>
          </div>
        ) : null}
      </div>
    </article>
  )
}

export function ImageGenerationHistory(props: ImageGenerationHistoryProps) {
  const { t } = useTranslation()

  return (
    <section className='border-border/70 min-w-0 rounded-lg border p-3'>
      <div className='mb-3 flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2 text-sm font-semibold'>
          <ImageIcon className='size-4' />
          {t('Image')}
        </div>
        <Button
          disabled={props.isRegenerating || props.items.length === 0}
          onClick={props.onClear}
          size='sm'
          type='button'
          variant='ghost'
        >
          <Trash2Icon />
          {t('Clear all')}
        </Button>
      </div>
      {props.items.length ? (
        <div className='grid gap-3'>
          {props.items.map((item) => (
            <ImageGenerationCard
              isRegenerating={props.isRegenerating}
              item={item}
              key={item.id}
              onDelete={() => props.onDelete(item.id)}
              onRegenerate={() => props.onRegenerate(item.id)}
            />
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground flex min-h-80 flex-col items-center justify-center gap-2 rounded-md border border-dashed p-8 text-center text-sm'>
          <ImageIcon className='size-9' />
          <strong className='text-foreground'>{t('No images yet')}</strong>
          <span>{t('Generated images appear here')}</span>
        </div>
      )}
    </section>
  )
}

export function VideoGenerationHistory(props: VideoGenerationHistoryProps) {
  const { t } = useTranslation()

  return (
    <section className='border-border/70 min-w-0 rounded-lg border p-3'>
      <div className='mb-3 flex items-center justify-between gap-3'>
        <div className='flex items-center gap-2 text-sm font-semibold'>
          <PlayIcon className='size-4' />
          {t('Video')}
        </div>
        <Button
          disabled={props.isRegenerating || props.tasks.length === 0}
          onClick={props.onClear}
          size='sm'
          type='button'
          variant='ghost'
        >
          <Trash2Icon />
          {t('Clear all')}
        </Button>
      </div>
      {props.tasks.length ? (
        <div className='grid gap-3'>
          {props.tasks.map((task) => (
            <VideoGenerationCard
              isRegenerating={props.isRegenerating}
              key={task.taskId}
              onDelete={() => props.onDelete(task.taskId)}
              onRegenerate={() => props.onRegenerate(task.taskId)}
              task={task}
            />
          ))}
        </div>
      ) : (
        <div className='text-muted-foreground flex min-h-80 flex-col items-center justify-center gap-2 rounded-md border border-dashed p-8 text-center text-sm'>
          <PlayIcon className='size-9' />
          <strong className='text-foreground'>{t('No videos yet')}</strong>
          <span>{t('Generated video appears here')}</span>
        </div>
      )}
    </section>
  )
}
