import { useRef, useEffect, useState } from "react";
import uPlot from "uplot";
import "uplot/dist/uPlot.min.css";

interface Props {
  opts: uPlot.Options;
  data: uPlot.AlignedData;
}

/**
 * Wrapper that measures its container, creates a uPlot instance at the
 * correct width, and resizes on container changes via ResizeObserver.
 *
 * Why not UplotReact? UplotReact creates the chart with width:1 and
 * relies on the caller to resize. By measuring the container first, we
 * avoid the initial 1px render flash.
 */
export default function AutoSizeUplot({ opts, data }: Props) {
  const containerRef = useRef<HTMLDivElement>(null);
  const chartRef = useRef<HTMLDivElement>(null);
  const uplotRef = useRef<uPlot | null>(null);
  const [width, setWidth] = useState(0);

  // Measure container on mount
  useEffect(() => {
    if (!containerRef.current) return;
    const ro = new ResizeObserver((entries) => {
      for (const entry of entries) {
        const w = Math.floor(entry.contentRect.width);
        if (w > 0) setWidth(w);
      }
    });
    ro.observe(containerRef.current);
    return () => ro.disconnect();
  }, []);

  // Create / recreate chart when opts, data, or width change
  useEffect(() => {
    if (width <= 0 || !chartRef.current) return;

    // Destroy previous instance
    if (uplotRef.current) {
      uplotRef.current.destroy();
      uplotRef.current = null;
    }

    const u = new uPlot(
      { ...opts, width, height: opts.height ?? 300 },
      data,
      chartRef.current
    );
    uplotRef.current = u;

    return () => {
      u.destroy();
      uplotRef.current = null;
    };
  }, [opts, data, width]);

  // Resize without recreating when only width changes
  useEffect(() => {
    if (uplotRef.current && width > 0) {
      uplotRef.current.setSize({
        width,
        height: opts.height ?? 300,
      });
    }
  }, [width, opts.height]);

  return (
    <div ref={containerRef} className="w-full">
      <div ref={chartRef} />
    </div>
  );
}
