import { NumberInput } from '@astryxdesign/core/NumberInput'
import { TextInput } from '@astryxdesign/core/TextInput'
import { TextArea } from '@astryxdesign/core/TextArea'
import { Selector } from '@astryxdesign/core/Selector'
import { Switch } from '@astryxdesign/core/Switch'
import { VStack } from '@astryxdesign/core/VStack'
import type { Camera, NodeFieldSpec } from '@/types/api'
import type { NodeFieldErrors } from '@/lib/validate-workflow'

export function NodeConfigFields({
  fields,
  values,
  cameras,
  errors,
  onChange,
  onCameraSelect,
}: {
  fields: NodeFieldSpec[]
  values: Record<string, unknown>
  cameras: Camera[]
  errors?: NodeFieldErrors
  onChange: (key: string, value: unknown) => void
  onCameraSelect: (camera: Camera) => void
}) {
  return (
    <VStack gap={3}>
      {fields.map((field) => {
        const fieldErr = errors?.[field.key]
        const status = fieldErr
          ? { type: 'error' as const, message: fieldErr }
          : undefined

        if (field.type === 'camera_info') {
          const current = values[field.key] as Camera | undefined
          return (
            <Selector
              key={field.key}
              label={field.label}
              description={field.description}
              value={current?.id != null ? String(current.id) : ''}
              status={status}
              onChange={(id) => {
                const cam = cameras.find((c) => String(c.id) === id)
                if (cam) onCameraSelect(cam)
              }}
              options={cameras.map((c) => ({
                label: c.name,
                value: String(c.id),
              }))}
            />
          )
        }
        if (field.type === 'select') {
          return (
            <Selector
              key={field.key}
              label={field.label}
              description={field.description}
              value={String(values[field.key] ?? '')}
              status={status}
              onChange={(v) => onChange(field.key, v)}
              options={field.options ?? []}
            />
          )
        }
        if (field.type === 'boolean') {
          return (
            <Switch
              key={field.key}
              label={field.label}
              description={field.description}
              value={Boolean(values[field.key])}
              onChange={(v) => onChange(field.key, v)}
            />
          )
        }
        if (field.type === 'number') {
          const num = values[field.key]
          return (
            <NumberInput
              key={field.key}
              label={field.label}
              description={field.description}
              value={typeof num === 'number' && !Number.isNaN(num) ? num : null}
              min={field.min ?? null}
              max={field.max ?? null}
              step={field.step ?? 0.01}
              status={status}
              onChange={(v) => onChange(field.key, v ?? '')}
            />
          )
        }
        if (field.type === 'json') {
          return (
            <TextArea
              key={field.key}
              label={field.label}
              description={field.description}
              status={status}
              value={
                typeof values[field.key] === 'string'
                  ? (values[field.key] as string)
                  : JSON.stringify(values[field.key] ?? {}, null, 2)
              }
              onChange={(v) => {
                try {
                  onChange(field.key, JSON.parse(v))
                } catch {
                  onChange(field.key, v)
                }
              }}
            />
          )
        }
        return (
          <TextInput
            key={field.key}
            label={field.label}
            description={field.description}
            status={status}
            value={String(values[field.key] ?? '')}
            onChange={(v) => onChange(field.key, v)}
          />
        )
      })}
    </VStack>
  )
}
