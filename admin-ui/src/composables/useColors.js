import { useLayoutStore } from '@/store/layout.store'

export default function useColors() {
  function makeLighter(color, ratio) {
    const r = Math.floor(parseInt(color.slice(1, 3), 16) * ratio)
    const g = Math.floor(parseInt(color.slice(3, 5), 16) * ratio)
    const b = Math.floor(parseInt(color.slice(5, 7), 16) * ratio)
    return `#${toHex(r)}${toHex(g)}${toHex(b)}`
  }

  function makeDarker(color, ratio) {
    const r = Math.floor(parseInt(color.slice(1, 3), 16) / ratio)
    const g = Math.floor(parseInt(color.slice(3, 5), 16) / ratio)
    const b = Math.floor(parseInt(color.slice(5, 7), 16) / ratio)
    return `#${toHex(r)}${toHex(g)}${toHex(b)}`
  }

  function buildThemeColorSeries(count) {
    const color = useLayoutStore().themeColor
    const series = []
    for (let i = 0; i < count; i++) {
      series.push(makeDarker(color, 1 + i * 0.2))
    }
    return series
  }

  function toHex(num) {
    const hex = num.toString(16)
    return hex.length === 1 ? `0${hex}` : hex
  }

  return { buildThemeColorSeries, makeLighter, makeDarker }
}
