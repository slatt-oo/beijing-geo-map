import { useCategories } from '../api/queries'
import { useAppStore } from '../store/useAppStore'
import { CategoryDot } from './CategoryDot'

export function CategoryFilter() {
  const { data: categories } = useCategories()
  const hidden = useAppStore((s) => s.hiddenCategories)
  const toggleCategory = useAppStore((s) => s.toggleCategory)

  if (!categories) return null
  return (
    <fieldset className="pointer-events-auto rounded-lg bg-white/95 p-2 shadow-lg ring-1 ring-slate-200">
      <legend className="sr-only">POI 分类</legend>
      {categories.map((c) => {
        const visible = !hidden.includes(c.code)
        return (
          <label key={c.code} className="flex cursor-pointer items-center gap-2 rounded px-2 py-1 text-sm hover:bg-slate-100">
            <input
              type="checkbox"
              checked={visible}
              onChange={() => toggleCategory(c.code)}
              style={{ accentColor: c.color }}
            />
            <CategoryDot color={c.color} />
            <span className={visible ? 'text-slate-800' : 'text-slate-400'}>{c.name}</span>
          </label>
        )
      })}
    </fieldset>
  )
}
