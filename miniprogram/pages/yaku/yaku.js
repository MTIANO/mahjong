const yakuData = require('../../data/yaku.js')

Page({
  data: {
    allYaku: yakuData,
    filteredYaku: yakuData,
    categories: ['1番', '2番', '3番', '6番', '役満'],
    activeCategory: '',
    searchText: ''
  },

  onSearch(e) {
    const text = e.detail.value.trim().toLowerCase()
    this.setData({ searchText: text })
    this.filterYaku()
  },

  onTabAll() {
    this.setData({ activeCategory: '' })
    this.filterYaku()
  },

  onTabChange(e) {
    const category = e.currentTarget.dataset.category
    this.setData({ activeCategory: category })
    this.filterYaku()
  },

  filterYaku() {
    const { allYaku, activeCategory, searchText } = this.data
    let filtered = allYaku

    if (activeCategory) {
      filtered = filtered.filter(y => y.category === activeCategory)
    }
    if (searchText) {
      filtered = filtered.filter(y =>
        y.name.toLowerCase().includes(searchText) ||
        y.nameJp.toLowerCase().includes(searchText) ||
        y.description.toLowerCase().includes(searchText)
      )
    }
    this.setData({ filteredYaku: filtered })
  }
})
