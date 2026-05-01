const app = getApp()

Page({
  data: {
    manTiles: ['1m','2m','3m','4m','5m','6m','7m','8m','9m'],
    pinTiles: ['1p','2p','3p','4p','5p','6p','7p','8p','9p'],
    souTiles: ['1s','2s','3s','4s','5s','6s','7s','8s','9s'],
    honorTiles: ['1z','2z','3z','4z','5z','6z','7z'],
    tileNames: {
      '1m':'一万','2m':'二万','3m':'三万','4m':'四万','5m':'五万','6m':'六万','7m':'七万','8m':'八万','9m':'九万',
      '1p':'一筒','2p':'二筒','3p':'三筒','4p':'四筒','5p':'五筒','6p':'六筒','7p':'七筒','8p':'八筒','9p':'九筒',
      '1s':'一索','2s':'二索','3s':'三索','4s':'四索','5s':'五索','6s':'六索','7s':'七索','8s':'八索','9s':'九索',
      '1z':'東','2z':'南','3z':'西','4z':'北','5z':'白','6z':'發','7z':'中'
    },
    meldTypeNames: { chi: '吃', pon: '碰', open_kan: '明杠', closed_kan: '暗杠' },
    selectedTiles: [],
    doraTiles: [],
    melds: [],
    meldBuffer: [],
    selectMode: 'hand',
    meldType: 'pon',
    isTsumo: true,
    isParent: true,
    seatWind: 0,
    roundWind: 0,
    winds: ['東','南','西','北'],
    loading: false,
    result: null,
    error: ''
  },

  getMaxHandTiles() {
    return 14 - this.data.melds.length * 3
  },

  addTile(e) {
    const tile = e.currentTarget.dataset.tile
    const mode = this.data.selectMode

    if (mode === 'dora') {
      const dora = this.data.doraTiles
      if (dora.length >= 5) {
        wx.showToast({ title: '最多5张宝牌', icon: 'none' })
        return
      }
      dora.push(tile)
      this.setData({ doraTiles: dora, result: null, error: '' })
    } else if (mode === 'meld') {
      this.addMeldTile(tile)
    } else {
      const selected = this.data.selectedTiles
      const max = this.getMaxHandTiles()
      if (selected.length >= max) {
        wx.showToast({ title: `手牌最多${max}张`, icon: 'none' })
        return
      }
      selected.push(tile)
      this.setData({ selectedTiles: selected, result: null, error: '' })
    }
  },

  addMeldTile(tile) {
    const buffer = this.data.meldBuffer
    const meldType = this.data.meldType
    const need = (meldType === 'open_kan' || meldType === 'closed_kan') ? 4 : 3

    buffer.push(tile)
    this.setData({ meldBuffer: buffer })

    if (buffer.length === need) {
      this.confirmMeld()
    }
  },

  confirmMeld() {
    const { meldBuffer, meldType, melds } = this.data
    const need = (meldType === 'open_kan' || meldType === 'closed_kan') ? 4 : 3

    if (meldBuffer.length !== need) {
      wx.showToast({ title: `需要选${need}张牌`, icon: 'none' })
      return
    }

    if (!this.validateMeld(meldBuffer, meldType)) {
      this.setData({ meldBuffer: [] })
      return
    }

    melds.push({ type: meldType, tiles: [...meldBuffer] })
    const max = 14 - melds.length * 3
    const selected = this.data.selectedTiles.slice(0, max)
    this.setData({ melds, meldBuffer: [], selectMode: 'hand', selectedTiles: selected, result: null, error: '' })
  },

  validateMeld(tiles, type) {
    if (type === 'pon') {
      if (tiles[0] !== tiles[1] || tiles[1] !== tiles[2]) {
        wx.showToast({ title: '碰需要3张相同牌', icon: 'none' })
        return false
      }
    } else if (type === 'open_kan' || type === 'closed_kan') {
      if (tiles[0] !== tiles[1] || tiles[1] !== tiles[2] || tiles[2] !== tiles[3]) {
        wx.showToast({ title: '杠需要4张相同牌', icon: 'none' })
        return false
      }
    } else if (type === 'chi') {
      const suit = tiles[0][1]
      if (suit === 'z') {
        wx.showToast({ title: '字牌不能吃', icon: 'none' })
        return false
      }
      for (let i = 1; i < tiles.length; i++) {
        if (tiles[i][1] !== suit) {
          wx.showToast({ title: '吃需要同花色', icon: 'none' })
          return false
        }
      }
      const nums = tiles.map(t => parseInt(t[0])).sort((a, b) => a - b)
      if (nums[1] - nums[0] !== 1 || nums[2] - nums[1] !== 1) {
        wx.showToast({ title: '吃需要连续数牌', icon: 'none' })
        return false
      }
    }
    return true
  },

  startMeld(e) {
    const type = e.currentTarget.dataset.type
    if (this.data.melds.length >= 4) {
      wx.showToast({ title: '最多4组副露', icon: 'none' })
      return
    }
    this.setData({ selectMode: 'meld', meldType: type, meldBuffer: [] })
  },

  cancelMeld() {
    this.setData({ selectMode: 'hand', meldBuffer: [] })
  },

  removeMeld(e) {
    const index = e.currentTarget.dataset.index
    const melds = this.data.melds
    melds.splice(index, 1)
    this.setData({ melds, result: null, error: '' })
  },

  removeTile(e) {
    const index = e.currentTarget.dataset.index
    const selected = this.data.selectedTiles
    selected.splice(index, 1)
    this.setData({ selectedTiles: selected, result: null, error: '' })
  },

  removeDora(e) {
    const index = e.currentTarget.dataset.index
    const dora = this.data.doraTiles
    dora.splice(index, 1)
    this.setData({ doraTiles: dora, result: null, error: '' })
  },

  setSelectMode(e) {
    const mode = e.currentTarget.dataset.mode
    this.setData({ selectMode: mode, meldBuffer: [] })
  },

  clearTiles() {
    this.setData({ selectedTiles: [], doraTiles: [], melds: [], meldBuffer: [], selectMode: 'hand', result: null, error: '' })
  },

  toggleTsumo() {
    this.setData({ isTsumo: !this.data.isTsumo })
  },

  toggleParent() {
    this.setData({ isParent: !this.data.isParent })
  },

  setSeatWind(e) {
    this.setData({ seatWind: parseInt(e.currentTarget.dataset.wind) })
  },

  setRoundWind(e) {
    this.setData({ roundWind: parseInt(e.currentTarget.dataset.wind) })
  },

  calculate() {
    const { selectedTiles, melds, doraTiles, isTsumo, isParent, seatWind, roundWind } = this.data
    const minHand = 14 - melds.length * 3 - 1
    if (selectedTiles.length < minHand) {
      wx.showToast({ title: '手牌数量不足', icon: 'none' })
      return
    }

    const tileStr = this.buildTileString(selectedTiles)
    const doraStr = doraTiles.length > 0 ? this.buildTileString(doraTiles) : ''
    const meldsData = melds.map(m => ({
      type: m.type,
      tiles: this.buildTileString(m.tiles)
    }))

    this.setData({ loading: true, error: '', result: null })

    wx.request({
      url: app.globalData.serverUrl + '/api/v1/calculate',
      method: 'POST',
      header: { 'Content-Type': 'application/json' },
      data: {
        tiles: tileStr,
        melds: meldsData,
        dora: doraStr,
        is_tsumo: isTsumo,
        is_parent: isParent,
        seat_wind: seatWind,
        round_wind: roundWind
      },
      success: (res) => {
        if (res.statusCode === 200) {
          this.setData({ result: res.data })
        } else {
          this.setData({ error: res.data.error || '计算失败' })
        }
      },
      fail: () => {
        this.setData({ error: '网络请求失败' })
      },
      complete: () => {
        this.setData({ loading: false })
      }
    })
  },

  buildTileString(tiles) {
    const groups = { m: [], p: [], s: [], z: [] }
    tiles.forEach(t => {
      const num = t[0]
      const suit = t[1]
      groups[suit].push(num)
    })
    let result = ''
    for (const suit of ['m', 'p', 's', 'z']) {
      if (groups[suit].length > 0) {
        result += groups[suit].join('') + suit
      }
    }
    return result
  }
})
