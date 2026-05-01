const scoreTable = {
  1: {
    30: { childRon: 1000, parentRon: 1500, childTsumo: [500, 300], parentTsumo: 500 },
    40: { childRon: 1300, parentRon: 2000, childTsumo: [700, 400], parentTsumo: 700 },
    50: { childRon: 1600, parentRon: 2400, childTsumo: [800, 400], parentTsumo: 800 },
    60: { childRon: 2000, parentRon: 2900, childTsumo: [1000, 500], parentTsumo: 1000 },
    70: { childRon: 2300, parentRon: 3400, childTsumo: [1200, 600], parentTsumo: 1200 },
    80: { childRon: 2600, parentRon: 3900, childTsumo: [1300, 700], parentTsumo: 1300 },
    90: { childRon: 2900, parentRon: 4400, childTsumo: [1500, 800], parentTsumo: 1500 },
    100: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    110: { childRon: 3600, parentRon: 5300, childTsumo: [1800, 900], parentTsumo: 1800 }
  },
  2: {
    25: { childRon: 1600, parentRon: 2400, childTsumo: [800, 400], parentTsumo: 800 },
    30: { childRon: 2000, parentRon: 2900, childTsumo: [1000, 500], parentTsumo: 1000 },
    40: { childRon: 2600, parentRon: 3900, childTsumo: [1300, 700], parentTsumo: 1300 },
    50: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    60: { childRon: 3900, parentRon: 5800, childTsumo: [2000, 1000], parentTsumo: 2000 },
    70: { childRon: 4500, parentRon: 6800, childTsumo: [2300, 1200], parentTsumo: 2300 },
    80: { childRon: 5200, parentRon: 7700, childTsumo: [2600, 1300], parentTsumo: 2600 },
    90: { childRon: 5800, parentRon: 8700, childTsumo: [2900, 1500], parentTsumo: 2900 },
    100: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    110: { childRon: 7100, parentRon: 10600, childTsumo: [3600, 1800], parentTsumo: 3600 }
  },
  3: {
    25: { childRon: 3200, parentRon: 4800, childTsumo: [1600, 800], parentTsumo: 1600 },
    30: { childRon: 3900, parentRon: 5800, childTsumo: [2000, 1000], parentTsumo: 2000 },
    40: { childRon: 5200, parentRon: 7700, childTsumo: [2600, 1300], parentTsumo: 2600 },
    50: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    60: { childRon: 7700, parentRon: 11600, childTsumo: [3900, 2000], parentTsumo: 3900 }
  },
  4: {
    25: { childRon: 6400, parentRon: 9600, childTsumo: [3200, 1600], parentTsumo: 3200 },
    30: { childRon: 7700, parentRon: 11600, childTsumo: [3900, 2000], parentTsumo: 3900 }
  },
  5: { childRon: 8000, parentRon: 12000, childTsumo: [4000, 2000], parentTsumo: 4000, name: "満貫" },
  6: { childRon: 12000, parentRon: 18000, childTsumo: [6000, 3000], parentTsumo: 6000, name: "跳満" },
  7: { childRon: 12000, parentRon: 18000, childTsumo: [6000, 3000], parentTsumo: 6000, name: "跳満" },
  8: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  9: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  10: { childRon: 16000, parentRon: 24000, childTsumo: [8000, 4000], parentTsumo: 8000, name: "倍満" },
  11: { childRon: 24000, parentRon: 36000, childTsumo: [12000, 6000], parentTsumo: 12000, name: "三倍満" },
  12: { childRon: 24000, parentRon: 36000, childTsumo: [12000, 6000], parentTsumo: 12000, name: "三倍満" },
  13: { childRon: 32000, parentRon: 48000, childTsumo: [16000, 8000], parentTsumo: 16000, name: "役満" }
}

module.exports = scoreTable
