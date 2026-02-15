import { useEffect, useMemo, useRef, useState } from "react";
import { invoke } from "@tauri-apps/api/core";
import * as echarts from "echarts";

type ForecastPoint = {
  date: string;
  balance: number;
};

function App() {
  const [points, setPoints] = useState<ForecastPoint[]>([]);
  const [error, setError] = useState<string>("");
  const [loading, setLoading] = useState(true);
  const chartRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    let mounted = true;

    invoke<ForecastPoint[]>("forecast_30_days")
      .then((result) => {
        if (!mounted) {
          return;
        }
        setPoints(result);
      })
      .catch((reason: unknown) => {
        if (!mounted) {
          return;
        }
        const message = reason instanceof Error ? reason.message : String(reason);
        setError(message);
      })
      .finally(() => {
        if (mounted) {
          setLoading(false);
        }
      });

    return () => {
      mounted = false;
    };
  }, []);

  const lastPoint = useMemo(
    () => (points.length > 0 ? points[points.length - 1] : undefined),
    [points]
  );

  useEffect(() => {
    if (!chartRef.current || points.length === 0) {
      return;
    }

    const chart = echarts.init(chartRef.current, undefined, {
      renderer: "canvas"
    });

    chart.setOption({
      animation: false,
      backgroundColor: "transparent",
      textStyle: {
        color: "#d6deed"
      },
      tooltip: {
        trigger: "axis",
        valueFormatter: (value: number | string) => `$${Number(value).toFixed(2)}`
      },
      grid: {
        top: 24,
        right: 30,
        bottom: 40,
        left: 70
      },
      xAxis: {
        type: "category",
        data: points.map((point) => point.date),
        boundaryGap: false,
        axisLine: {
          lineStyle: {
            color: "#6b7a99"
          }
        }
      },
      yAxis: {
        type: "value",
        axisLabel: {
          formatter: (value: number | string) => `$${Number(value).toFixed(0)}`
        },
        splitLine: {
          lineStyle: {
            color: "rgba(151, 170, 204, 0.15)"
          }
        }
      },
      series: [
        {
          name: "Forecast",
          type: "line",
          smooth: true,
          showSymbol: false,
          lineStyle: {
            width: 3,
            color: "#6dd5ff"
          },
          areaStyle: {
            color: new echarts.graphic.LinearGradient(0, 0, 0, 1, [
              { offset: 0, color: "rgba(109, 213, 255, 0.35)" },
              { offset: 1, color: "rgba(109, 213, 255, 0.02)" }
            ])
          },
          data: points.map((point) => point.balance)
        }
      ]
    });

    const onResize = () => {
      chart.resize();
    };

    window.addEventListener("resize", onResize);

    return () => {
      window.removeEventListener("resize", onResize);
      chart.dispose();
    };
  }, [points]);

  return (
    <main className="dashboard">
      <section className="panel headline">
        <div>
          <h1 className="title">Aurum Forecast</h1>
          <p className="subtle">30-day liquid cash projection from local SQLite data</p>
        </div>
        <p className="subtle">
          {lastPoint ? `Day 30: $${lastPoint.balance.toFixed(2)}` : "Loading..."}
        </p>
      </section>

      <section className="panel">
        {loading ? <p className="subtle">Calculating forecast...</p> : null}
        {error ? <div className="error">Forecast failed: {error}</div> : null}
        <div className="chart" ref={chartRef} />
      </section>
    </main>
  );
}

export default App;
