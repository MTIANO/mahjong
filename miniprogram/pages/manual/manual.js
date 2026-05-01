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
    selectedTiles: [],
    doraTiles: [],
    selectingDora: false,
    isTsumo: true,
    seatWind: 0,
    roundWind: 0,
    winds: ['東','南','西','北'],
    loading: false,
    result: null,
    error: ''
  },

  addTile(e) {
    const tile = e.currentTarget.dataset.tile
    if (this.data.selectingDora) {
      const dora = this.data.doraTiles
      if (dora.length >= 5) {
        wx.showToast({ title: '最多5张宝牌', icon: 'none' })
        return
      }
      dora.push(tile)
      this.setData({ doraTiles: dora, result: null, error: '' })
    } else {
      const selected = this.data.selectedTiles
      if (selected.length >= 14) {
        wx.showToast({ title: '最多选14张牌', icon: 'none' })
        return
      }
      selected.push(tile)
      this.setData({ selectedTiles: selected, result: null, error: '' })
    }
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

  toggleDoraMode() {
    this.setData({ selectingDora: !this.data.selectingDora })
  },

  clearTiles() {
    this.setData({ selectedTiles: [], doraTiles: [], result: null, error: '' })
  },

  toggleTsumo() {
    this.setData({ isTsumo: !this.data.isTsumo })
  },

  setSeatWind(e) {
    this.setData({ seatWind: parseInt(e.currentTarget.dataset.wind) })
  },

  setRoundWind(e) {
    this.setData({ roundWind: parseInt(e.currentTarget.dataset.wind) })
  },

  calculate() {
    const { selectedTiles, doraTiles, isTsumo, seatWind, roundWind } = this.data
    if (selectedTiles.length < 13) {
      wx.showToast({ title: '至少选13张牌', icon: 'none' })
      return
    }

    const tileStr = this.buildTileString(selectedTiles)
    const doraStr = doraTiles.length > 0 ? this.buildTileString(doraTiles) : ''
    this.setData({ loading: true, error: '', result: null })

    wx.request({
      url: app.globalData.serverUrl + '/api/v1/calculate',
      method: 'POST',
      header: { 'Content-Type': 'application/json' },
      data: {
        tiles: tileStr,
        dora: doraStr,
        is_tsumo: isTsumo,
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
