const app = getApp()

Page({
  data: {
    imagePath: '',
    loading: false,
    result: null,
    error: ''
  },

  chooseImage() {
    wx.chooseMedia({
      count: 1,
      mediaType: ['image'],
      sourceType: ['album', 'camera'],
      success: (res) => {
        this.setData({
          imagePath: res.tempFiles[0].tempFilePath,
          result: null,
          error: ''
        })
      }
    })
  },

  recognize() {
    if (!this.data.imagePath) return

    this.setData({ loading: true, error: '' })

    wx.uploadFile({
      url: app.globalData.serverUrl + '/api/v1/recognize',
      filePath: this.data.imagePath,
      name: 'image',
      success: (res) => {
        try {
          const data = JSON.parse(res.data)
          if (res.statusCode === 200) {
            this.setData({ result: data })
          } else {
            this.setData({ error: data.error || '识别失败' })
          }
        } catch (e) {
          this.setData({ error: '解析响应失败' })
        }
      },
      fail: () => {
        this.setData({ error: '网络请求失败，请检查后端服务是否启动' })
      },
      complete: () => {
        this.setData({ loading: false })
      }
    })
  }
})
