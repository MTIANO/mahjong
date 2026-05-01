const scoreTable = require('../../data/score.js')

Page({
  data: {
    hanOptions: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13],
    fuOptions: [20, 25, 30, 40, 50, 60, 70, 80, 90, 100, 110],
    hanIndex: 0,
    fuIndex: 2,
    isParent: false,
    showFu: true,
    score: null
  },

  onLoad() {
    this.calculateScore()
  },

  onHanChange(e) {
    const hanIndex = parseInt(e.detail.value)
    const han = this.data.hanOptions[hanIndex]
    this.setData({
      hanIndex,
      showFu: han <= 4
    })
    this.calculateScore()
  },

  onFuChange(e) {
    this.setData({ fuIndex: parseInt(e.detail.value) })
    this.calculateScore()
  },

  onParentChange(e) {
    this.setData({ isParent: e.detail.value })
    this.calculateScore()
  },

  calculateScore() {
    const han = this.data.hanOptions[this.data.hanIndex]
    const fu = this.data.fuOptions[this.data.fuIndex]

    const hanEntry = scoreTable[han]
    if (!hanEntry) {
      this.setData({ score: null })
      return
    }

    let score
    if (han >= 5) {
      score = hanEntry
    } else {
      score = hanEntry[fu]
    }

    this.setData({ score: score || null })
  }
})
