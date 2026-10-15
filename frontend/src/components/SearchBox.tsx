import { useState, type KeyboardEvent } from 'react'
import { useCategories, useSearch } from '../api/queries'
import type { SearchHit } from '../api/types'
import { useDebounce } from '../hooks/useDebounce'
import { FALLBACK_POI_COLOR } from '../lib/categories'
import { MAX_QUERY_LENGTH, toSearchTerm } from '../lib/search'
import { focusPoi } from '../map/focus'
import { useMap } from '../map/MapView'

const SEARCH_DEBOUNCE_MS = 300

export function SearchBox() {
  const map = useMap()
  const [text, setText] = useState('')
  const [chosen, setChosen] = useState(false)
  const [open, setOpen] = useState(false)
  const [active, setActive] = useState(-1)
  // 选中结果后输入框显示它的名称，但不再用这个名称搜索：有些名称超过接口允许的 50 个字符。
  const query = useDebounce(chosen ? '' : toSearchTerm(text), SEARCH_DEBOUNCE_MS)
  const { data: hits, isFetching } = useSearch(query)
  const { data: categories } = useCategories()

  const colors = new Map(categories?.map((c) => [c.code, c.color]))
  // 清空输入后的 300ms 内 query 还是旧值，这时不能再用键盘选中旧结果。
  const hasText = text.trim() !== ''
  const results = hasText && query !== '' ? (hits ?? []) : []
  const showDropdown = open && hasText && query !== '' && hits !== undefined
  // 新结果可能比旧结果少，键盘选中的位置超出范围时视为没有选中。
  const activeIndex = active < results.length ? active : -1

  const choose = (hit: SearchHit) => {
    setText(hit.name)
    setChosen(true)
    setOpen(false)
    setActive(-1)
    focusPoi(map, hit.id, hit.location)
  }

  const onKeyDown = (e: KeyboardEvent<HTMLInputElement>) => {
    // 输入法组字时的回车用来确认输入，不是选择结果（Safari 中 isComposing 为 false，只能看 keyCode）。
    if (e.nativeEvent.isComposing || e.keyCode === 229) return
    if (e.key === 'ArrowDown' && results.length > 0) {
      e.preventDefault()
      setOpen(true)
      setActive((activeIndex + 1) % results.length)
    } else if (e.key === 'ArrowUp' && results.length > 0) {
      e.preventDefault()
      setActive(activeIndex <= 0 ? results.length - 1 : activeIndex - 1)
    } else if (e.key === 'Enter' && results.length > 0) {
      e.preventDefault()
      choose(results[Math.max(activeIndex, 0)])
    } else if (e.key === 'Escape') {
      // 阻止搜索框的默认行为：Chrome 和 Safari 按 Esc 会清空输入。
      e.preventDefault()
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
        aria-autocomplete="list"
        aria-expanded={showDropdown}
        aria-controls={showDropdown ? 'search-results' : undefined}
        aria-activedescendant={showDropdown && activeIndex >= 0 ? `search-option-${results[activeIndex].id}` : undefined}
        onChange={(e) => {
          setText(e.target.value)
          setChosen(false)
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
              id={`search-option-${hit.id}`}
              role="option"
              aria-selected={i === activeIndex}
              // 用 mousedown 而不是 click：click 之前输入框会先失焦，下拉框已经关掉了。
              onMouseDown={(e) => {
                e.preventDefault()
                choose(hit)
              }}
              onMouseEnter={() => setActive(i)}
              className={`flex cursor-pointer items-center gap-2 px-3 py-1.5 text-sm ${i === activeIndex ? 'bg-slate-100' : ''}`}
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
