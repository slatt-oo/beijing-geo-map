import { create } from 'zustand'

export type AppState = {
  hiddenCategories: string[]
  selectedDistrict: string | null
  activePoiId: number | null
  errorMessage: string | null
  toggleCategory: (code: string) => void
  selectDistrict: (adcode: string | null) => void
  openPoi: (id: number | null) => void
  showError: (message: string) => void
  dismissError: () => void
}

export const useAppStore = create<AppState>()((set) => ({
  hiddenCategories: [],
  selectedDistrict: null,
  activePoiId: null,
  errorMessage: null,
  toggleCategory: (code) =>
    set((state) => ({
      hiddenCategories: state.hiddenCategories.includes(code)
        ? state.hiddenCategories.filter((c) => c !== code)
        : [...state.hiddenCategories, code],
    })),
  selectDistrict: (adcode) => set({ selectedDistrict: adcode }),
  openPoi: (id) => set({ activePoiId: id }),
  showError: (message) => set({ errorMessage: message }),
  dismissError: () => set({ errorMessage: null }),
}))
