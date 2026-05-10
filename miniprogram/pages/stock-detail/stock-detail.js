const auth = require('../../utils/auth')

Page({
  data: {
    quote: null,
    chartType: 'minute',
    chartLoading: true,
    volumeText: '--',
    amountText: '--',
    marketCapText: '--'
  },

  minuteData: null,
  dailyData: null,
  canvas: null,
  ctx: null,
  dpr: 1,
  touchInfo: null,

  onLoad(options) {
    this.stockCode = options.code || ''
    this.stockName = options.name || ''
    if (this.stockName) {
      wx.setNavigationBarTitle({ title: this.stockName })
    }
    this.dpr = wx.getWindowInfo().pixelRatio
    this.loadData()
  },

  loadData() {
    this.loadQuote()
    this.loadMinuteKline()
    this.loadDailyKline()
  },

  loadQuote() {
    auth.authRequest({
      url: '/api/v1/stock/quote/' + this.stockCode,
      method: 'GET'
    }).then(res => {
      if (res.statusCode === 200 && res.data.quote) {
        const q = res.data.quote
        this.setData({
          quote: q,
          volumeText: this.formatVolume(q.volume),
          amountText: this.formatAmount(q.amount),
          marketCapText: this.formatMarketCap(q.market_cap)
        })
      }
    }).catch(err => console.error('load quote:', err))
  },

  loadMinuteKline() {
    auth.authRequest({
      url: '/api/v1/stock/kline/minute/' + this.stockCode,
      method: 'GET'
    }).then(res => {
      if (res.statusCode === 200) {
        this.minuteData = res.data
        if (this.data.chartType === 'minute') {
          this.drawChart()
        }
      }
    }).catch(err => console.error('load minute:', err))
  },

  loadDailyKline() {
    auth.authRequest({
      url: '/api/v1/stock/kline/daily/' + this.stockCode,
      method: 'GET'
    }).then(res => {
      if (res.statusCode === 200 && res.data.klines) {
        this.dailyData = res.data.klines
        if (this.data.chartType === 'daily') {
          this.drawChart()
        }
      }
    }).catch(err => console.error('load daily:', err))
  },

  switchChart(e) {
    const type = e.currentTarget.dataset.type
    if (type === this.data.chartType) return
    this.setData({ chartType: type })
    this.drawChart()
  },

  drawChart() {
    if (!this.canvas) {
      this.initCanvas().then(() => this.renderChart())
    } else {
      this.renderChart()
    }
  },

  initCanvas() {
    return new Promise(resolve => {
      const query = wx.createSelectorQuery()
      query.select('#stockChart')
        .fields({ node: true, size: true })
        .exec(res => {
          if (!res[0]) {
            this.setData({ chartLoading: false })
            return
          }
          const canvas = res[0].node
          const ctx = canvas.getContext('2d')
          const dpr = this.dpr
          canvas.width = res[0].width * dpr
          canvas.height = res[0].height * dpr
          ctx.scale(dpr, dpr)
          this.canvas = canvas
          this.ctx = ctx
          this.canvasWidth = res[0].width
          this.canvasHeight = res[0].height
          resolve()
        })
    })
  },

  renderChart() {
    if (!this.ctx) return
    this.setData({ chartLoading: true })
    const ctx = this.ctx
    const w = this.canvasWidth
    const h = this.canvasHeight

    ctx.clearRect(0, 0, w, h)

    if (this.data.chartType === 'minute') {
      this.drawMinuteChart(ctx, w, h)
    } else {
      this.drawDailyChart(ctx, w, h)
    }
    this.setData({ chartLoading: false })
  },

  drawMinuteChart(ctx, w, h) {
    if (!this.minuteData || !this.minuteData.data || this.minuteData.data.length === 0) return

    const data = this.minuteData.data
    const prevClose = this.minuteData.prev_close || data[0].price
    const padding = { left: 55, right: 10, top: 15, bottom: 40 }
    const chartW = w - padding.left - padding.right
    const priceH = (h - padding.top - padding.bottom) * 0.7
    const volH = (h - padding.top - padding.bottom) * 0.25
    const volTop = padding.top + priceH + (h - padding.top - padding.bottom) * 0.05

    let minP = prevClose, maxP = prevClose
    let maxVol = 0
    for (const d of data) {
      if (d.price < minP) minP = d.price
      if (d.price > maxP) maxP = d.price
      if (d.avg_price < minP) minP = d.avg_price
      if (d.avg_price > maxP) maxP = d.avg_price
      if (d.volume > maxVol) maxVol = d.volume
    }
    const diff = Math.max(maxP - prevClose, prevClose - minP, prevClose * 0.01)
    minP = prevClose - diff
    maxP = prevClose + diff

    const priceY = p => padding.top + (1 - (p - minP) / (maxP - minP)) * priceH
    const xStep = chartW / 240

    // 背景网格
    ctx.strokeStyle = '#f0f0f0'
    ctx.lineWidth = 0.5
    for (let i = 0; i <= 4; i++) {
      const y = padding.top + (priceH / 4) * i
      ctx.beginPath()
      ctx.moveTo(padding.left, y)
      ctx.lineTo(padding.left + chartW, y)
      ctx.stroke()
    }

    // 昨收虚线
    const pcY = priceY(prevClose)
    ctx.strokeStyle = '#999'
    ctx.lineWidth = 0.5
    ctx.setLineDash([4, 3])
    ctx.beginPath()
    ctx.moveTo(padding.left, pcY)
    ctx.lineTo(padding.left + chartW, pcY)
    ctx.stroke()
    ctx.setLineDash([])

    // Y 轴标签
    ctx.fillStyle = '#999'
    ctx.font = '10px sans-serif'
    ctx.textAlign = 'right'
    ctx.fillText(maxP.toFixed(2), padding.left - 4, padding.top + 4)
    ctx.fillText(prevClose.toFixed(2), padding.left - 4, pcY + 3)
    ctx.fillText(minP.toFixed(2), padding.left - 4, padding.top + priceH + 4)

    // X 轴时间标签
    ctx.textAlign = 'center'
    ctx.fillText('9:30', padding.left, h - padding.bottom + 16)
    ctx.fillText('11:30', padding.left + chartW * 0.5, h - padding.bottom + 16)
    ctx.fillText('15:00', padding.left + chartW, h - padding.bottom + 16)

    // 成交量柱状图
    if (maxVol > 0) {
      const barW = Math.max(chartW / 240 - 0.5, 0.5)
      for (let i = 0; i < data.length; i++) {
        const d = data[i]
        const x = padding.left + i * xStep
        const bh = (d.volume / maxVol) * volH
        ctx.fillStyle = d.price >= prevClose ? 'rgba(255,77,79,0.4)' : 'rgba(82,196,26,0.4)'
        ctx.fillRect(x, volTop + volH - bh, barW, bh)
      }
    }

    // 均价线
    ctx.strokeStyle = '#ff9800'
    ctx.lineWidth = 1
    ctx.beginPath()
    let started = false
    for (let i = 0; i < data.length; i++) {
      const x = padding.left + i * xStep
      const y = priceY(data[i].avg_price)
      if (!started) { ctx.moveTo(x, y); started = true }
      else ctx.lineTo(x, y)
    }
    ctx.stroke()

    // 价格线 + 填充
    ctx.strokeStyle = '#1890ff'
    ctx.lineWidth = 1.5
    ctx.beginPath()
    for (let i = 0; i < data.length; i++) {
      const x = padding.left + i * xStep
      const y = priceY(data[i].price)
      if (i === 0) ctx.moveTo(x, y)
      else ctx.lineTo(x, y)
    }
    ctx.stroke()

    // 面积填充
    const lastX = padding.left + (data.length - 1) * xStep
    ctx.lineTo(lastX, pcY)
    ctx.lineTo(padding.left, pcY)
    ctx.closePath()
    ctx.fillStyle = 'rgba(24,144,255,0.08)'
    ctx.fill()
  },

  drawDailyChart(ctx, w, h) {
    if (!this.dailyData || this.dailyData.length === 0) return

    const data = this.dailyData
    const padding = { left: 55, right: 10, top: 15, bottom: 40 }
    const chartW = w - padding.left - padding.right
    const candleH = (h - padding.top - padding.bottom) * 0.7
    const volH = (h - padding.top - padding.bottom) * 0.25
    const volTop = padding.top + candleH + (h - padding.top - padding.bottom) * 0.05

    let minP = Infinity, maxP = -Infinity, maxVol = 0
    for (const d of data) {
      if (d.low < minP) minP = d.low
      if (d.high > maxP) maxP = d.high
      if (d.volume > maxVol) maxVol = d.volume
    }
    const pRange = maxP - minP || 1
    const priceY = p => padding.top + (1 - (p - minP) / pRange) * candleH

    const n = data.length
    const candleW = Math.max((chartW / n) * 0.7, 1)
    const gap = chartW / n

    // 背景网格
    ctx.strokeStyle = '#f0f0f0'
    ctx.lineWidth = 0.5
    for (let i = 0; i <= 4; i++) {
      const y = padding.top + (candleH / 4) * i
      ctx.beginPath()
      ctx.moveTo(padding.left, y)
      ctx.lineTo(padding.left + chartW, y)
      ctx.stroke()
    }

    // Y 轴标签
    ctx.fillStyle = '#999'
    ctx.font = '10px sans-serif'
    ctx.textAlign = 'right'
    for (let i = 0; i <= 4; i++) {
      const p = maxP - (pRange / 4) * i
      const y = padding.top + (candleH / 4) * i
      ctx.fillText(p.toFixed(2), padding.left - 4, y + 4)
    }

    // X 轴日期标签
    ctx.textAlign = 'center'
    const labelCount = Math.min(5, n)
    for (let i = 0; i < labelCount; i++) {
      const idx = Math.floor((i / (labelCount - 1)) * (n - 1))
      const d = data[idx]
      const x = padding.left + idx * gap + gap / 2
      ctx.fillText(d.date.slice(5), x, h - padding.bottom + 16)
    }

    // MA 均线
    this.drawMA(ctx, data, 5, '#ff9800', padding, gap, priceY)
    this.drawMA(ctx, data, 10, '#9c27b0', padding, gap, priceY)

    // 蜡烛图 + 成交量
    for (let i = 0; i < n; i++) {
      const d = data[i]
      const x = padding.left + i * gap + (gap - candleW) / 2
      const cx = padding.left + i * gap + gap / 2
      const isUp = d.close >= d.open
      const color = isUp ? '#ff4d4f' : '#52c41a'

      // 蜡烛体
      const bodyTop = priceY(Math.max(d.open, d.close))
      const bodyBot = priceY(Math.min(d.open, d.close))
      const bodyH = Math.max(bodyBot - bodyTop, 1)

      ctx.fillStyle = color
      ctx.fillRect(x, bodyTop, candleW, bodyH)

      // 上下影线
      ctx.strokeStyle = color
      ctx.lineWidth = 1
      ctx.beginPath()
      ctx.moveTo(cx, priceY(d.high))
      ctx.lineTo(cx, bodyTop)
      ctx.moveTo(cx, bodyBot)
      ctx.lineTo(cx, priceY(d.low))
      ctx.stroke()

      // 成交量柱
      if (maxVol > 0) {
        const vh = (d.volume / maxVol) * volH
        ctx.fillStyle = isUp ? 'rgba(255,77,79,0.5)' : 'rgba(82,196,26,0.5)'
        ctx.fillRect(x, volTop + volH - vh, candleW, vh)
      }
    }

    // MA 图例
    ctx.font = '10px sans-serif'
    ctx.textAlign = 'left'
    ctx.fillStyle = '#ff9800'
    ctx.fillText('MA5', padding.left + 4, padding.top - 2)
    ctx.fillStyle = '#9c27b0'
    ctx.fillText('MA10', padding.left + 44, padding.top - 2)
  },

  drawMA(ctx, data, period, color, padding, gap, priceY) {
    if (data.length < period) return
    ctx.strokeStyle = color
    ctx.lineWidth = 1
    ctx.beginPath()
    let started = false
    for (let i = period - 1; i < data.length; i++) {
      let sum = 0
      for (let j = i - period + 1; j <= i; j++) sum += data[j].close
      const ma = sum / period
      const x = padding.left + i * gap + gap / 2
      const y = priceY(ma)
      if (!started) { ctx.moveTo(x, y); started = true }
      else ctx.lineTo(x, y)
    }
    ctx.stroke()
  },

  onChartTouch(e) {
    if (!this.ctx || !e.touches[0]) return
    const touch = e.touches[0]
    const x = touch.x
    const padding = { left: 55, right: 10 }
    const chartW = this.canvasWidth - padding.left - padding.right

    if (x < padding.left || x > padding.left + chartW) return

    const ratio = (x - padding.left) / chartW
    let label = ''

    if (this.data.chartType === 'minute' && this.minuteData && this.minuteData.data.length > 0) {
      const idx = Math.min(Math.floor(ratio * this.minuteData.data.length), this.minuteData.data.length - 1)
      const d = this.minuteData.data[idx]
      label = d.time + '  ' + d.price.toFixed(2)
    } else if (this.data.chartType === 'daily' && this.dailyData && this.dailyData.length > 0) {
      const idx = Math.min(Math.floor(ratio * this.dailyData.length), this.dailyData.length - 1)
      const d = this.dailyData[idx]
      label = d.date + '  O:' + d.open + ' C:' + d.close + ' H:' + d.high + ' L:' + d.low
    }

    if (label) {
      this.renderChart()
      const ctx = this.ctx
      // 十字光标
      ctx.strokeStyle = '#999'
      ctx.lineWidth = 0.5
      ctx.setLineDash([3, 2])
      ctx.beginPath()
      ctx.moveTo(x, 15)
      ctx.lineTo(x, this.canvasHeight - 40)
      ctx.stroke()
      ctx.setLineDash([])

      // 信息标签
      ctx.fillStyle = 'rgba(26,26,46,0.85)'
      const tw = ctx.measureText(label).width + 16
      const lx = Math.min(x - tw / 2, this.canvasWidth - tw - 4)
      ctx.fillRect(Math.max(lx, 4), 0, tw, 16)
      ctx.fillStyle = '#fff'
      ctx.font = '10px sans-serif'
      ctx.textAlign = 'left'
      ctx.fillText(label, Math.max(lx, 4) + 8, 12)
    }
  },

  onChartTouchEnd() {
    this.renderChart()
  },

  formatVolume(vol) {
    if (!vol) return '--'
    if (vol >= 100000000) return (vol / 100000000).toFixed(2) + '亿'
    if (vol >= 10000) return (vol / 10000).toFixed(2) + '万'
    return vol + ''
  },

  formatAmount(amount) {
    if (!amount) return '--'
    if (amount >= 10000) return (amount / 10000).toFixed(2) + '亿'
    return amount.toFixed(2) + '万'
  },

  formatMarketCap(cap) {
    if (!cap) return '--'
    return cap.toFixed(2) + '亿'
  }
})
