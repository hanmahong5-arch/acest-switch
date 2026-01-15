import fallbackIcons from './fallbackLobeIcons'

const globIcons = import.meta.glob('../../node_modules/@lobehub/icons-static-svg/icons/*.svg', {
  eager: true,
  import: 'default',
  query: '?raw',
}) as Record<string, string>

const normalize = (source: Record<string, string>) => {
  return Object.entries(source).reduce<Record<string, string>>((acc, [key, value]) => {
    const name = key
      .split('/')
      .pop()
      ?.replace('.svg', '')
      ?.toLowerCase()
    if (name) {
      acc[name] = value
    }
    return acc
  }, {})
}

const normalizedFallback = Object.keys(fallbackIcons).reduce<Record<string, string>>((acc, key) => {
  acc[key.toLowerCase()] = fallbackIcons[key]
  return acc
}, {})

const normalizedGlob = normalize(globIcons)

const lobeIconMap: Record<string, string> = {
  ...normalizedFallback,
  ...normalizedGlob,
}

export default lobeIconMap
