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
import type { ImageGenerationRequest } from '../types'

export function createImageEditFormData(
  payload: ImageGenerationRequest,
  images: File[]
): FormData {
  const formData = new FormData()
  formData.append('model', payload.model)
  if (payload.group) formData.append('group', payload.group)
  formData.append('prompt', payload.prompt)
  formData.append('n', String(payload.n))
  formData.append('size', payload.size)
  if (payload.quality) formData.append('quality', payload.quality)
  images.forEach((image) => formData.append('image[]', image, image.name))
  return formData
}
