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
import { FilmIcon, LoaderCircleIcon, SparklesIcon, UploadIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'

import { ModelGroupSelector } from '@/components/model-group-selector'
import { Button } from '@/components/ui/button'
import { Label } from '@/components/ui/label'
import { NativeSelect, NativeSelectOption } from '@/components/ui/native-select'
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

const completedStatuses = new Set(['succeeded', 'success', 'completed'])
const failedStatuses = new Set(['failed', 'error', 'cancelled', 'canceled'])

export function VideoWorkspace({
  config,
  groups,
  isModelLoading,
  models,
  onConfigChange,
}: VideoWorkspaceProps) {
  const { t } = useTranslation()
  const inputRef = useRef<HTMLInputElement>(null)
  const [prompt, setPrompt] = useState('')
  const [reference, setReference] = useState<string | null>(null)
  const [ratio, setRatio] = useState('16:9')
  const [duration, setDuration] = useState(5)
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
          toast.error(error instanceof Error ? error.message : t('Request failed'))
        })
    }, 3000)

    return () => window.clearInterval(timer)
  }, [isCompleted, isFailed, t, taskId])

  const handleReferenceFile = (file: File | undefined) => {
    if (!file) return

    const reader = new FileReader()
    reader.onload = () => setReference(String(reader.result))
    reader.onerror = () => toast.error(t('Unable to read image'))
    reader.readAsDataURL(file)
  }

  const handleGenerate = async () => {
    if (!prompt.trim() || !config.model) return

    const [width, height] = ratio === '9:16' ? [720, 1280] : [1280, 720]
    setIsSubmitting(true)
    setTask(null)
    try {
      const response = await createVideo({
        model: config.model,
        group: config.group,
        prompt: prompt.trim(),
        duration,
        width,
        height,
        n: 1,
        ...(reference ? { image: reference } : {}),
      })
      setTask(response)
    } catch (error) {
      toast.error(error instanceof Error ? error.message : t('Request failed'))
    } finally {
      setIsSubmitting(false)
    }
  }

  const videoURL =
    task?.url || (isCompleted && taskId ? `/v1/videos/${taskId}/content` : null)

  return (
    <div className='grid min-h-0 flex-1 overflow-auto lg:grid-cols-[22rem_minmax(0,1fr)]'>
      <section className='border-border/70 bg-muted/15 flex min-h-0 flex-col border-b p-4 lg:border-r lg:border-b-0'>
        <div className='space-y-4 overflow-y-auto pr-1'>
          <ModelGroupSelector
            disabled={isSubmitting || isModelLoading}
            groups={groups}
            models={models}
            onGroupChange={(value) => onConfigChange('group', value)}
            onModelChange={(value) => onConfigChange('model', value)}
            selectedGroup={config.group}
            selectedModel={config.model}
          />
          <div className='space-y-2'>
            <Label htmlFor='video-prompt'>{t('Prompt')}</Label>
            <Textarea
              disabled={isSubmitting}
              id='video-prompt'
              onChange={(event) => setPrompt(event.target.value)}
              placeholder={t('Describe the video you want to create')}
              value={prompt}
            />
          </div>
          <div className='grid grid-cols-2 gap-3'>
            <label className='space-y-2 text-sm font-medium'>
              {t('Aspect ratio')}
              <NativeSelect
                disabled={isSubmitting}
                onChange={(event) => setRatio(event.target.value)}
                value={ratio}
              >
                <NativeSelectOption value='16:9'>16:9</NativeSelectOption>
                <NativeSelectOption value='9:16'>9:16</NativeSelectOption>
              </NativeSelect>
            </label>
            <label className='space-y-2 text-sm font-medium'>
              {t('Duration')}
              <NativeSelect
                disabled={isSubmitting}
                onChange={(event) => setDuration(Number(event.target.value))}
                value={duration}
              >
                {[5, 10].map((value) => (
                  <NativeSelectOption key={value} value={value}>
                    {t('{{count}} seconds', { count: value })}
                  </NativeSelectOption>
                ))}
              </NativeSelect>
            </label>
          </div>
          <div className='space-y-2'>
            <Label>{t('Reference image')}</Label>
            <input
              accept='image/*'
              className='sr-only'
              onChange={(event) => {
                handleReferenceFile(event.target.files?.[0])
                event.currentTarget.value = ''
              }}
              ref={inputRef}
              type='file'
            />
            {reference ? (
              <img
                alt={t('Reference image')}
                className='h-28 w-full rounded-lg border object-cover'
                src={reference}
              />
            ) : null}
            <Button
              className='w-full'
              disabled={isSubmitting}
              onClick={() => inputRef.current?.click()}
              type='button'
              variant='outline'
            >
              <UploadIcon />
              {t('Upload')}
            </Button>
          </div>
        </div>
        <Button
          className='mt-4 w-full'
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

      <section className='flex min-h-[24rem] min-w-0 flex-1 items-center justify-center p-4 md:p-6'>
        {videoURL ? (
          <video
            className='max-h-[70vh] max-w-full rounded-lg border bg-black'
            controls
            src={videoURL}
          />
        ) : (
          <div className='text-muted-foreground flex flex-col items-center gap-3 text-center text-sm'>
            {taskId && !isFailed ? (
              <LoaderCircleIcon className='size-9 animate-spin' />
            ) : (
              <FilmIcon className='size-9' />
            )}
            <span>
              {isFailed
                ? task?.error?.message || t('Video generation failed')
                : taskId
                  ? t('Waiting for video...')
                  : t('Generated video appears here')}
            </span>
          </div>
        )}
      </section>
    </div>
  )
}
