import { MapContainer, TileLayer, Polyline } from "react-leaflet";
import "leaflet/dist/leaflet.css";
import { WorkoutRoute } from "../../api";
import { useMemo } from "react";
import type { LatLngTuple, LatLngBoundsExpression } from "leaflet";

interface Props {
  route: WorkoutRoute[];
}

export default function RouteMap({ route }: Props) {
  const { positions, bounds } = useMemo(() => {
    const positions: LatLngTuple[] = route.map((p) => [
      p.Latitude,
      p.Longitude,
    ]);

    if (positions.length === 0) return { positions, bounds: null };

    let minLat = Infinity,
      maxLat = -Infinity,
      minLng = Infinity,
      maxLng = -Infinity;
    for (const [lat, lng] of positions) {
      if (lat < minLat) minLat = lat;
      if (lat > maxLat) maxLat = lat;
      if (lng < minLng) minLng = lng;
      if (lng > maxLng) maxLng = lng;
    }

    const bounds: LatLngBoundsExpression = [
      [minLat - 0.001, minLng - 0.001],
      [maxLat + 0.001, maxLng + 0.001],
    ];

    return { positions, bounds };
  }, [route]);

  if (positions.length === 0 || !bounds) return null;

  return (
    <div className="bg-zinc-900 border border-zinc-800 rounded-lg p-4">
      <h3 className="text-sm font-medium text-zinc-400 mb-3">Route</h3>
      <div className="rounded-lg overflow-hidden h-80">
        <MapContainer
          bounds={bounds}
          scrollWheelZoom={true}
          style={{ height: "100%", width: "100%" }}
        >
          <TileLayer
            attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a>'
            url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
          />
          <Polyline
            positions={positions}
            pathOptions={{ color: "#22d3ee", weight: 3, opacity: 0.8 }}
          />
        </MapContainer>
      </div>
    </div>
  );
}
