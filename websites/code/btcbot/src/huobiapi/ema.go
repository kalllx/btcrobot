/*
  btcbot is a Bitcoin trading bot for HUOBI.com written
  in golang, it features multiple trading methods using
  technical analysis.

  Disclaimer:

  USE AT YOUR OWN RISK!

  The author of this project is NOT responsible for any damage or loss caused
  by this software. There can be bugs and the bot may not perform as expected
  or specified. Please consider testing it first with paper trading /
  backtesting on historical data. Also look at the code to see what how
  it's working.

  Weibo:http://weibo.com/bocaicfa
*/

package huobiapi

import (
	. "config"
	"fmt"
	"logger"
	"os"
	"service"
	"strconv"
)

//for backup the kline file to detect the huobi bug
func (w *Huobi) backup(Time string) {

	oldFile := "cache/TradeKLine_minute.data"
	newFile := fmt.Sprintf("%s_%s", oldFile, Time)
	os.Rename(oldFile, newFile)
}

func (w *Huobi) checkException(yPrevPrice, Price, Volumn float64) bool {
	if Price > yPrevPrice+10 && Volumn < 1 {
		return false
	}

	if Price < yPrevPrice-10 && Volumn < 1 {
		return false
	}

	return true
}
func (w *Huobi) doEMA(Time []string, Price []float64, Volumn []float64) {
	if len(Price) == 0 {
		logger.Errorln("no data is prepared!")
		return
	}
	//read config
	shortEMA, _ := strconv.Atoi(Option["shortEMA"])
	longEMA, _ := strconv.Atoi(Option["longEMA"])

	stopPoints, err := strconv.ParseFloat(Config["stopPoints"], 64)
	if err != nil {
		logger.Debugln("config item stopPoints is not float")
		return
	}

	EMAMinThreshold, err := strconv.ParseFloat(Config["EMAMinThreshold"], 64)
	if err != nil {
		logger.Debugln("config item EMAMinThreshold is not float")
		return
	}

	tradeOnlyAfterSwitch, _ := strconv.Atoi(Config["tradeOnlyAfterSwitch"])
	TresholdLevel, _ := strconv.Atoi(Config["TresholdLevel"])
	_, err = strconv.ParseFloat(Option["tradeAmount"], 64)
	if err != nil {
		logger.Debugln("config item tradeAmount is not float")
		return
	}
	tradeAmount := Option["tradeAmount"]

	f_tradeAmount, err := strconv.ParseFloat(Option["tradeAmount"], 64)
	if err != nil {
		logger.Debugln("config item tradeAmount is not float")
		return
	}

	MACDtradeAmount := fmt.Sprintf("%0.02f", f_tradeAmount/2.0)

	//compute the indictor
	emaShort := EMA(Price, shortEMA)
	emaLong := EMA(Price, longEMA)
	EMAdif := getEMAdif(emaShort, emaLong)

	length := len(Price)

	//check indictor using history data, loop through data, get history samples
	logger.OverrideStart(w.Peroid)
	logger.Overridef("EMA 收益率分析[%d:[s=%d/l=%d],stopPoints:%0.0f]\n", w.Peroid, shortEMA, longEMA, stopPoints)
	var profit float64
	var times int
	var lastTrade float64
	var entryPrice float64
	var totaltimes int

	if w.Disable_trading != 1 {
		if Config["env"] == "dev" {
			logger.Infof("%s\t%-6.2f\t%-6.2f\n", Time[length-1], Price[length-1], Volumn[length-1])
		}
	}

	//EMA cross
	for i := 1; i < length; i++ {
		if EMAdif[i-1] < 0 && EMAdif[i] > 0 { //up cross
			w.lastAction = "buyIn"
			totaltimes++
			logger.Overrideln(totaltimes)

			if times == 0 {
				entryPrice = Price[i]
			}
			times++
			profit -= Price[i]
			lastTrade = Price[i]

			w.lastBuyprice = Price[i]

			var samplesBegin int
			if i > longEMA {
				samplesBegin = i - longEMA
			} else {
				samplesBegin = 0
			}
			periodArr := Price[samplesBegin:i]

			w.lastLowestprice = arrayLowest(periodArr)

			if EMAdif[i] >= EMAMinThreshold {
				logger.Overrideln("++ buyIn", i, Time[i], Price[i], fmt.Sprintf("%0.04f", EMAdif[i]), w.lastLowestprice, 2*w.lastBuyprice-w.lastLowestprice)
			} else {
				logger.Overrideln(" + buyIn", i, Time[i], Price[i], fmt.Sprintf("%0.04f", EMAdif[i]), w.lastLowestprice, 2*w.lastBuyprice-w.lastLowestprice)
			}

			if w.checkException(Price[i-1], Price[i], Volumn[i]) == false {
				logger.Infoln("detect exception data in huobi.com", Price[i-1], Price[i], Volumn[i])
				continue
			}
			if i == length-1 && w.latestMACDTrend != 1 {
				if w.Disable_trading != 1 {
					logger.Infoln("EMA up cross, 买入buyIn", w.getTradePrice(""))
					go service.TriggerTrender("EMA up cross, 买入buyIn")
				}

				if Option["disable_trading"] != "1" && w.Disable_trading != 1 {
					w.latestMACDTrend = 1
					w.Do_buy(w.getTradePrice("buy"), tradeAmount)
				}

				w.backup(Time[i])
			}
		} else if (w.lastAction != "sellOut" || w.lastAction != "stop") && EMAdif[i-1] > 0 && EMAdif[i] < 0 { //down cross
			w.lastAction = "sellOut"
			if EMAdif[i] <= -EMAMinThreshold {
				logger.Overrideln("-- sellOut", i, Time[i], Price[i], fmt.Sprintf("%0.04f", EMAdif[i]))
			} else {
				logger.Overrideln(" - sellOut", i, Time[i], Price[i], fmt.Sprintf("%0.04f", EMAdif[i]))
			}

			if w.checkException(Price[i-1], Price[i], Volumn[i]) == false {
				logger.Infoln("detect exception data in huobi.com", Price[i-1], Price[i], Volumn[i])
				continue
			}

			if i == length-1 && w.latestMACDTrend != -1 {
				if w.Disable_trading != 1 {
					logger.Infoln("EMA down cross, 卖出sellOut", w.getTradePrice(""))
					go service.TriggerTrender("EMA down cross, 卖出sellOut")
				}
				if Option["disable_trading"] != "1" && w.Disable_trading != 1 {
					w.latestMACDTrend = -1
					ret := w.Do_sell(w.getTradePrice("sell"), tradeAmount)
					if ret == false {
						w.Do_sell(w.getTradePrice("sell"), MACDtradeAmount)
					}
				}

				w.backup(Time[i])
			}

			if times != 0 {
				times++
				profit += Price[i]
				lastTrade = Price[i]
				if times != 0 && times%2 == 0 {
					logger.Overridef("profit=%0.02f, rate=%0.02f%%\n", Price[i]-w.lastBuyprice, 100*(Price[i]-w.lastBuyprice)/w.lastBuyprice)
				}
			}
		} /* else if (w.lastAction != "exit" || w.lastAction != "stop") && Price[i] < emaMiddle[i]-stopPoints { //stop
			w.lastAction = "stop"
			logger.Overrideln("-- stop", i, Time[i], Price[i], fmt.Sprintf("%0.04f", emaMiddle[i]))
			if i == length-1 && w.latestMACDTrend != -1 {
				logger.Infoln("保守止损位", w.getTradePrice(""))
				go service.TriggerTrender("保守止损位")

				ret := w.do_sell(w.getTradePrice("sell"), tradeAmount)
				if ret == false {
					w.do_sell(w.getTradePrice("sell"), MACDtradeAmount)
				}
			}

			if times != 0 {
				times++
				profit += Price[i]
				lastTrade = Price[i]
				if times != 0 && times%2 == 0 {
					logger.Overridef("profit=%0.02f, rate=%0.02f%%\n", Price[i]-w.lastBuyprice, 100*(Price[i]-w.lastBuyprice)/w.lastBuyprice)
				}
			}
		}*/

	}

	if times%2 != 0 {
		profit += lastTrade
		totaltimes--
	}
	logger.Overridef("totaltimes[%d] profit=%0.02f, entryPrice=%0.02f, rate=%0.02f%%\n", totaltimes, profit, entryPrice, 100*profit/entryPrice)

	if false {
		//current trend
		//trade according trend indictor
		if w.latestSolidTrend == 0 {
			w.findLatestSolidTrend(emaShort, emaLong, EMAMinThreshold,
				TresholdLevel, length)
		}

		w.trade(emaShort, emaLong, EMAMinThreshold,
			TresholdLevel, length, tradeOnlyAfterSwitch, tradeAmount)
	}
}

func getEMAdifAt(emaShort, emaLong []float64, idx int) float64 {
	var cel = emaLong[idx]
	var ces = emaShort[idx]
	if cel == 0 {
		return 0
	} else {
		return 100 * (ces - cel) / ((ces + cel) / 2)
	}
}

func getEMAdif(emaShort, emaLong []float64) []float64 {
	// loop through data
	var EMAdifs []float64
	length := len(emaShort)
	for i := 0; i < length; i++ {
		EMAdifAt := getEMAdifAt(emaShort, emaLong, i)
		EMAdifs = append(EMAdifs, EMAdifAt)
	}

	return EMAdifs
}

/* Function based on the idea of an exponential moving average.
 *
 * Formula: EMA = Price(t) * k + EMA(y) * (1 - k)
 * t = today y = yesterday N = number of days in EMA k = 2/(2N+1)
 *
 * @param Price : array of y variables.
 * @param Time : array of x variables.
 * @param periods : The amount of "days" to average from.
 * @return an array containing the EMA.
**/
func EMA(Price []float64, periods int) []float64 {

	var t float64
	y := 0.0
	n := float64(periods)
	var k float64
	k = 2 / (n + 1)
	var ema float64 // exponential moving average.

	var periodArr []float64
	var startpos int
	length := len(Price)
	var emLine []float64 = make([]float64, length)

	// loop through data
	for i := 0; i < length; i++ {
		if Price[i] != 0 {
			startpos = i + 1
			break
		} else {
			emLine[i] = 0
		}
	}

	for i := startpos; i < length; i++ {
		periodArr = append(periodArr, Price[i])

		// 0: runs if the periodArr has enough points.
		// 1: set currentvalue (today).
		// 2: set last value. either by past avg or yesterdays ema.
		// 3: calculate todays ema.
		if periods == len(periodArr) {

			t = Price[i]

			if y == 0 {
				y = arrayAvg(periodArr)
			} else {
				ema = (t * k) + (y * (1 - k))
				y = ema
			}

			emLine[i] = y

			// remove first value in array.
			periodArr = periodArr[1:]

		} else {

			emLine[i] = 0
		}

	}

	return emLine
}

/* Function that returns average of an array's values.
 *
**/
func arrayAvg(arr []float64) float64 {
	sum := 0.0

	for i := 0; i < len(arr); i++ {
		sum = sum + arr[i]
	}

	return (sum / (float64)(len(arr)))
}
