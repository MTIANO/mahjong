const auth = require('../../utils/auth')

Page({
  data: {
    today: '',
    activeTab: '',
    recommendations: [],
    loading: false,
    inputCode: ''
  },

  onShow() {
    this.setData({ today: this.formatDate(new Date()) })
    this.ensureLoginAndLoad()
  },

  onPullDownRefresh() {
    this.loadRecommendations().then(() => wx.stopPullDownRefresh())
  },

  formatDate(date) {
    const y = date.getFullYear()
    const m = String(date.getMonth() + 1).padStart(2, '0')
    const d = String(date.getDate()).padStart(2, '0')
    return y + '-' + m + '-' + d
  },

  ensureLoginAndLoad() {
    if (auth.isLoggedIn()) {
      this.loadRecommendations()
      return
    }
    this.setData({ loading: true })
    auth.login().then(() => {
      this.loadRecommendations()
    }).catch(err => {
      console.error('login failed:', err)
      wx.showToast({ title: '登录失败', icon: 'none' })
      this.setData({ loading: false })
    })
  },

  loadRecommendations() {
    this.setData({ loading: true })
    const source = this.data.activeTab
    return auth.authRequest({
      url: '/api/v1/stock/recommendations' + (source ? '?source=' + source : ''),
      method: 'GET'
    }).then(res => {
      this.setData({
        recommendations: res.data.recommendations || [],
        loading: false
      })
    }).catch(err => {
      console.error('load recommendations failed:', err)
      if (err.message === 'token expired' || err.message === 'not logged in') {
        auth.login().then(() => this.loadRecommendations())
        return
      }
      this.setData({ loading: false })
      wx.showToast({ title: '加载失败', icon: 'none' })
    })
  },

  switchTab(e) {
    const tab = e.currentTarget.dataset.tab
    this.setData({ activeTab: tab })
    this.loadRecommendations()
  },

  onInputCode(e) {
    this.setData({ inputCode: e.detail.value.trim() })
  },

  goDetail(e) {
    const { code, name } = e.currentTarget.dataset
    wx.navigateTo({
      url: '/pages/stock-detail/stock-detail?code=' + code + '&name=' + encodeURIComponent(name)
    })
  },

  addWatchlist() {
    const code = this.data.inputCode
    if (!code) return

    auth.authRequest({
      url: '/api/v1/stock/watchlist',
      method: 'POST',
      data: { stock_code: code }
    }).then(res => {
      if (res.statusCode === 200) {
        wx.showToast({ title: '添加成功' })
        this.setData({ inputCode: '' })
        this.loadRecommendations()
      } else {
        wx.showToast({ title: res.data.error || '添加失败', icon: 'none' })
      }
    }).catch(() => {
      wx.showToast({ title: '网络错误', icon: 'none' })
    })
  }
})
