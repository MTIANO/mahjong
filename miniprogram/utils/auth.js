const app = getApp()

function getToken() {
  return wx.getStorageSync('token') || ''
}

function setToken(token) {
  wx.setStorageSync('token', token)
}

function clearToken() {
  wx.removeStorageSync('token')
}

function isLoggedIn() {
  return !!getToken()
}

function login() {
  return new Promise((resolve, reject) => {
    wx.login({
      success(res) {
        if (!res.code) {
          reject(new Error('wx.login failed'))
          return
        }
        wx.request({
          url: app.globalData.serverUrl + '/api/v1/auth/login',
          method: 'POST',
          data: { code: res.code },
          success(resp) {
            if (resp.statusCode === 200 && resp.data.token) {
              setToken(resp.data.token)
              resolve(resp.data)
            } else {
              reject(new Error(resp.data.error || 'login failed'))
            }
          },
          fail(err) {
            reject(err)
          }
        })
      },
      fail(err) {
        reject(err)
      }
    })
  })
}

function authRequest(options) {
  const token = getToken()
  if (!token) {
    return Promise.reject(new Error('not logged in'))
  }
  return new Promise((resolve, reject) => {
    wx.request({
      ...options,
      url: app.globalData.serverUrl + options.url,
      header: {
        ...options.header,
        'Authorization': 'Bearer ' + token
      },
      success(res) {
        if (res.statusCode === 401) {
          clearToken()
          reject(new Error('token expired'))
          return
        }
        resolve(res)
      },
      fail: reject
    })
  })
}

module.exports = { getToken, setToken, clearToken, isLoggedIn, login, authRequest }
