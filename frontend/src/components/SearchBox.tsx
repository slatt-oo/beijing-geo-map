import { useState, type KeyboardEvent } from 'react'
import { useCategories, useSearch } from '../api/queries'
import type { SearchHit } from '../api/types'
import { useDebounce } from '../hooks/useDebounce'
import { FALLBACK_POI_COLOR } from '../lib/categories'
import { focusPoi } from '../map/focus'
import { useMap } from '../map/MapView'

const SEARCH_DEBOUNCE_MS = 300
const MAX_QUERY_LENGTH = 50

export function SearchBox() {
  const map = useMap()
  const [text, setText] = useState('')
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  const query = useDebounce(text.trim(), SEARCH_DEBOUNCE_MS)
  const { data: hits, isFetching } = useSearch(query)
  const { data: categories } = useCategories()

  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  const results = query === '' ? [] : (hits ?? [])
  const showDropdown = open && query !== '' && hits !== undefined

  const choose = (hit: SearchHit) => {
    setText(hit.name)
    setOpen(false)
    setActive(-1)
    focusPoi(map, hit.id, hit.location)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown' && results.length > 0) {
      e.preventDefault()
      setOpen(true)
      setActive((i) => (i + 1) % results.length)
    } else if (e.key === 'ArrowUp' && results.length > 0) {
      e.preventDefault()
      setActive((i) => (i <= 0 ? results.length - 1 : i - 1))
    } else if (e.key === 'Enter' && results.length > 0) {
      e.preventDefault()
      choose(results[Math.max(active, 0)])
    } else if (e.key === 'Escape') {
      setOpen(false)
    }
  }

  return (
    <div className="pointer-events-auto relative">
      <input
        type="search"
        value={text}
        maxLength={MAX_QUERY_LENGTH}
        placeholder="搜索地铁站、高校、景点…"
        aria-label="搜索地点"
        role="combobox"
        aria-expanded={showDropdown}
        aria-controls="search-results"
        onChange={(e) => {
          setText(e.target.value)
          setOpen(true)
          setActive(-1)
        }}
        onFocus={() => setOpen(true)}
        onBlur={() => setOpen(false)}
        onKeyDown={onKeyDown}
        className="w-full rounded-lg bg-white/95 px-3 py-2 text-sm text-slate-900 shadow-lg ring-1 ring-slate-200 outline-none placeholder:text-slate-400 focus:ring-2 focus:ring-blue-400"
      />
      {showDropdown && (
        <ul
          id="search-results"
          role="listbox"
          className="absolute left-0 right-0 top-full z-20 mt-1 max-h-96 overflow-y-auto rounded-lg bg-white py-1 shadow-lg ring-1 ring-slate-200"
        >
          {results.length === 0 && (
            <li className="px-3 py-2 text-sm text-slate-500">{isFetching ? '搜索中…' : '没有找到相关地点'}</li>
          )}
          {results.map((hit, i) => (
            <li
              key={hit.id}
              role="option"
              aria-selected={i === active}
              // 用 mousedown 而不是 click：click 之前输入框会先失焦，下拉框已经关掉了。
              onMouseDown={(e) => {
                e.preventDefault()
                choose(hit)
              }}
              onMouseEnter={() => setActive(i)}
              className={`flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm ${i === active ? 'bg-slate-100' : ''}`}
            >
              <span
                className="h-2.5 w-2.5 shrink-0 rounded-full"
                style={{ backgroundColor: colors.get(hit.category) ?? FALLBACK_POI_COLOR }}
              />
              <span className="flex-1 truncate text-slate-900">{hit.name}</span>
              {hit.district_name && <span className="shrink-0 text-xs text-slate-500">{hit.district_name}</span>}
            </li>
          ))}
        </ul>
      )}
    </div>
  )
}
