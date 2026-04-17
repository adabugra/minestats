<script lang="ts">
    import { onMount } from "svelte";
    import {
        IconBrandGithub,
        IconDeviceDesktop,
        IconMoon,
        IconServer,
        IconSun,
        IconTrendingUp,
        IconUsers,
    } from "@tabler/icons-svelte";
    import CombinedChart from "./lib/charts/CombinedChart.svelte";
    import ServerMiniChart from "./lib/charts/ServerMiniChart.svelte";

    type ThemeMode = "light" | "dark" | "system";

    type Sample = {
        server_id: string;
        ts_ms: number;
        online_players: number | null;
        max_players: number | null;
        latency_ms: number | null;
        is_online: boolean;
        motd?: string;
        version?: string;
        favicon?: string;
        error?: string;
    };

    type ServerInfo = {
        id: string;
        name: string;
        address: string;
        refresh_seconds: number;
        last_sample: Sample | null;
    };

    type RangeOption = {
        key: "15m" | "1h" | "6h" | "24h" | "7d" | "all";
        label: string;
        minutes: number;
    };

    type RankedServer = {
        server: ServerInfo;
        originalIndex: number;
        online: number | null;
    };

    const ranges: RangeOption[] = [
        { key: "15m", label: "15m", minutes: 15 },
        { key: "1h", label: "1h", minutes: 60 },
        { key: "6h", label: "6h", minutes: 360 },
        { key: "24h", label: "24h", minutes: 1440 },
        { key: "7d", label: "7d", minutes: 10080 },
        { key: "all", label: "all", minutes: 10080 },
    ];

    const palette = [
        "#10b981",
        "#3b82f6",
        "#f59e0b",
        "#8b5cf6",
        "#ec4899",
        "#14b8a6",
        "#f97316",
        "#6366f1",
    ];
    const FAVICON_CACHE_MS = 15 * 60 * 1000;
    const MINI_HISTORY_MINUTES = 60;
    const STATS_HISTORY_MINUTES = 24 * 60;
    const MINI_HISTORY_TTL_MS = 8 * 1000;
    const STATS_HISTORY_TTL_MS = 30 * 1000;
    const COMBINED_HISTORY_TTL_MS = 15 * 1000;
    const COMBINED_LINE_BREAK_GAP_MS = 15 * 60 * 1000;

    let servers: ServerInfo[] = [];
    let statsSeries: Record<string, [number, number | null][]> = {};
    let miniSeries: Record<string, [number, number | null][]> = {};
    let combinedSeriesSource: Record<string, [number, number | null][]> = {};
    let errorMsg = "";
    let selectedRange: RangeOption["key"] = "6h";
    let loading = true;
    let hasLoadedOnce = false;
    let faviconCache: Record<string, { value: string; updatedAt: number }> = {};
    let historyCache: Record<
        number,
        {
            fetchedAt: number;
            series: Record<string, [number, number | null][]>;
        }
    > = {};

    let themeMode: ThemeMode = "system";
    let isDark = false;

    const apiBase = import.meta.env.VITE_API_BASE ?? "http://localhost:8080";

    function applyTheme(mode: ThemeMode) {
        themeMode = mode;
        const prefersDark = window.matchMedia(
            "(prefers-color-scheme: dark)",
        ).matches;
        isDark = mode === "dark" || (mode === "system" && prefersDark);
        document.documentElement.classList.toggle("dark", isDark);
        window.localStorage.setItem("minestats-theme", mode);
    }

    function cycleTheme() {
        const next: ThemeMode =
            themeMode === "light"
                ? "dark"
                : themeMode === "dark"
                  ? "system"
                  : "light";
        applyTheme(next);
    }

    function modeLabel(mode: ThemeMode) {
        if (mode === "light") return "Light";
        if (mode === "dark") return "Dark";
        return "System";
    }

    function activeMinutes() {
        return (
            ranges.find((range) => range.key === selectedRange)?.minutes ?? 360
        );
    }

    function formatServerAddress(address: string) {
        const [host, rawPort] = String(address).split(":");
        if (!rawPort || rawPort === "25565") return host;
        return `${host}:${rawPort}`;
    }

    function normalizeKey(value: string) {
        return String(value)
            .toLowerCase()
            .replace(/[^a-z0-9]/g, "");
    }

    function resolveSeriesRows(
        rowsByServer: Record<string, [number, number | null][]>,
        server: ServerInfo,
        index = 0,
    ): [number, number | null][] {
        const byID = rowsByServer[server.id];
        if (byID) return byID;

        const serverIDNormalized = normalizeKey(server.id);
        const serverNameNormalized = normalizeKey(server.name);
        for (const [key, rows] of Object.entries(rowsByServer)) {
            const normalized = normalizeKey(key);
            if (
                normalized === serverIDNormalized ||
                normalized === serverNameNormalized ||
                normalized.includes(serverIDNormalized) ||
                serverIDNormalized.includes(normalized) ||
                normalized.includes(serverNameNormalized) ||
                serverNameNormalized.includes(normalized)
            ) {
                return rows;
            }
        }

        return Object.values(rowsByServer)[index] ?? [];
    }

    function latestSeriesValue(server: ServerInfo, index = 0): number | null {
        const rows = resolveSeriesRows(statsSeries, server, index);
        return latestNumericValue(rows);
    }

    function latestNumericValue(
        rows: [number, number | null][],
    ): number | null {
        for (let i = rows.length - 1; i >= 0; i -= 1) {
            const value = rows[i]?.[1];
            if (typeof value === "number") return value;
        }
        return null;
    }

    function numericRows(rows: [number, number | null][]) {
        return rows.filter(
            (point): point is [number, number] => typeof point[1] === "number",
        );
    }

    function rowsWithBreaksForGaps(
        rows: [number, number | null][],
        maxGapMs: number,
    ): [number, number | null][] {
        if (rows.length < 2) return rows;

        const out: [number, number | null][] = [];
        let previousTs = rows[0][0];
        out.push(rows[0]);

        for (let i = 1; i < rows.length; i += 1) {
            const point = rows[i];
            const [ts] = point;
            if (ts - previousTs > maxGapMs) {
                out.push([previousTs + 1, null]);
            }
            out.push(point);
            previousTs = ts;
        }

        return out;
    }

    function latestSampleValue(server: ServerInfo): number | null {
        const sample = server.last_sample;
        if (!sample || !sample.is_online) return null;
        const value = sample.online_players;
        if (typeof value === "number") return value;
        const parsed = Number(value);
        return Number.isFinite(parsed) ? parsed : null;
    }

    function currentOnline(server: ServerInfo, index = 0): number | null {
        return latestSeriesValue(server, index) ?? latestSampleValue(server);
    }

    function rankServersByOnline(currentServers: ServerInfo[]): RankedServer[] {
        const ranked = currentServers.map((server, originalIndex) => ({
            server,
            originalIndex,
            online: currentOnline(server, originalIndex),
        }));

        ranked.sort((a, b) => {
            const aOnline = a.online;
            const bOnline = b.online;
            const aIsNum = typeof aOnline === "number";
            const bIsNum = typeof bOnline === "number";

            if (aIsNum && bIsNum && aOnline !== bOnline) return bOnline - aOnline;
            if (aIsNum !== bIsNum) return aIsNum ? -1 : 1;
            return a.server.name.localeCompare(b.server.name);
        });

        return ranked;
    }

    function totalOnline(currentServers: ServerInfo[]) {
        return currentServers.reduce((sum, server, index) => {
            const value = currentOnline(server, index);
            return sum + (typeof value === "number" ? value : 0);
        }, 0);
    }

    function totalPeak24h(currentServers: ServerInfo[]) {
        const cutoff = Date.now() - 24 * 60 * 60 * 1000;
        const totalsByTs = new Map<number, number>();

        for (const [index, server] of currentServers.entries()) {
            const rows = resolveSeriesRows(statsSeries, server, index);
            for (const [ts, value] of rows) {
                if (ts < cutoff || typeof value !== "number") continue;
                totalsByTs.set(ts, (totalsByTs.get(ts) ?? 0) + value);
            }
        }

        let peak = 0;
        for (const total of totalsByTs.values()) {
            if (total > peak) peak = total;
        }
        return peak;
    }

    function status(server: ServerInfo, index: number) {
        const current = currentOnline(server, index);
        if (typeof current !== "number") return "timed out";
        return `${current}`;
    }

    function peak24h(server: ServerInfo, index: number) {
        const cutoff = Date.now() - 24 * 60 * 60 * 1000;
        const rows = resolveSeriesRows(statsSeries, server, index);
        let max = -1;
        for (const [ts, value] of rows) {
            if (ts >= cutoff && typeof value === "number" && value > max)
                max = value;
        }
        return max >= 0 ? max : null;
    }

    function topRecord(server: ServerInfo, index: number) {
        const rows = resolveSeriesRows(statsSeries, server, index);
        let best: { value: number; ts: number } | null = null;
        for (const [ts, value] of rows) {
            if (typeof value !== "number") continue;
            if (!best || value > best.value) best = { value, ts };
        }
        return best;
    }

    function formatTopRecordTime(ts: number) {
        return new Intl.DateTimeFormat("en-GB", {
            year: "numeric",
            day: "2-digit",
            month: "2-digit",
            hour: "2-digit",
            minute: "2-digit",
            hour12: false,
        }).format(new Date(ts));
    }

    function serverColor(index: number) {
        return palette[index % palette.length];
    }

    function normalizeHistorySeries(rawSeries: Record<string, unknown[]>) {
        const normalizedSeries: Record<string, [number, number | null][]> = {};
        for (const [serverID, rows] of Object.entries(rawSeries)) {
            normalizedSeries[serverID] = (rows ?? [])
                .map((row) => {
                    if (!Array.isArray(row) || row.length < 2) return null;
                    const ts = Number(row[0]);
                    if (!Number.isFinite(ts)) return null;
                    const rawValue = row[1];
                    if (rawValue === null || rawValue === undefined)
                        return [ts, null] as [number, null];
                    const value = Number(rawValue);
                    return Number.isFinite(value)
                        ? ([ts, value] as [number, number])
                        : ([ts, null] as [number, null]);
                })
                .filter(
                    (point): point is [number, number | null] => point !== null,
                );
        }
        return normalizedSeries;
    }

    function applyFaviconCache(incomingServers: ServerInfo[], nowMs: number) {
        const nextCache = { ...faviconCache };
        const stabilized = incomingServers.map((server) => {
            const sample = server.last_sample;
            if (!sample) return server;

            const cached = nextCache[server.id];
            const cacheValid =
                !!cached && nowMs - cached.updatedAt < FAVICON_CACHE_MS;
            const incomingFavicon =
                typeof sample.favicon === "string" && sample.favicon.length > 0
                    ? sample.favicon
                    : null;

            let faviconToUse: string | null = incomingFavicon;
            if (cacheValid && cached) {
                faviconToUse = cached.value;
            } else if (incomingFavicon) {
                nextCache[server.id] = {
                    value: incomingFavicon,
                    updatedAt: nowMs,
                };
                faviconToUse = incomingFavicon;
            } else if (cached) {
                faviconToUse = cached.value;
            }

            if ((sample.favicon ?? null) === faviconToUse) return server;

            return {
                ...server,
                last_sample: {
                    ...sample,
                    favicon: faviconToUse ?? undefined,
                },
            };
        });

        faviconCache = nextCache;
        return stabilized;
    }

    async function fetchHistory(minutes: number) {
        const historyRes = await fetch(
            `${apiBase}/api/history?minutes=${minutes}`,
        );
        if (!historyRes.ok) throw new Error("API request failed");
        const historyJson = await historyRes.json();
        const rawSeries = (historyJson.series ?? {}) as Record<
            string,
            unknown[]
        >;
        return normalizeHistorySeries(rawSeries);
    }

    async function fetchHistoryCached(minutes: number, ttlMs: number) {
        const nowMs = Date.now();
        const cached = historyCache[minutes];
        if (cached && nowMs - cached.fetchedAt < ttlMs) {
            return cached.series;
        }

        const series = await fetchHistory(minutes);
        const fetchedAt = Date.now();
        historyCache = {
            ...historyCache,
            [minutes]: {
                fetchedAt,
                series,
            },
        };
        return series;
    }

    async function refresh() {
        try {
            if (!hasLoadedOnce) loading = true;
            const [serversRes, fixedStatsSeries, fixedMiniSeries, rangeSeries] =
                await Promise.all([
                    fetch(`${apiBase}/api/servers`),
                    fetchHistoryCached(
                        STATS_HISTORY_MINUTES,
                        STATS_HISTORY_TTL_MS,
                    ),
                    fetchHistoryCached(
                        MINI_HISTORY_MINUTES,
                        MINI_HISTORY_TTL_MS,
                    ),
                    fetchHistoryCached(
                        activeMinutes(),
                        COMBINED_HISTORY_TTL_MS,
                    ),
                ]);
            if (!serversRes.ok) throw new Error("API request failed");

            const serversJson = await serversRes.json();

            const incomingServers = (serversJson.servers ?? []) as ServerInfo[];
            servers = applyFaviconCache(incomingServers, Date.now());
            combinedSeriesSource = rangeSeries;
            statsSeries = fixedStatsSeries;
            miniSeries = fixedMiniSeries;
            errorMsg = "";
        } catch (err) {
            errorMsg = err instanceof Error ? err.message : "Unknown error";
        } finally {
            loading = false;
            hasLoadedOnce = true;
        }
    }

    async function refreshCombinedSeries() {
        try {
            combinedSeriesSource = await fetchHistoryCached(
                activeMinutes(),
                COMBINED_HISTORY_TTL_MS,
            );
            errorMsg = "";
        } catch (err) {
            errorMsg = err instanceof Error ? err.message : "Unknown error";
        }
    }

    async function setRange(next: RangeOption["key"]) {
        if (selectedRange === next) return;
        selectedRange = next;
        await refreshCombinedSeries();
    }

    $: rawCombinedSeries = Object.entries(combinedSeriesSource).map(
        ([key, data], index) => ({
            name:
                servers.find(
                    (server) => normalizeKey(server.id) === normalizeKey(key),
                )?.name ?? key,
            color: serverColor(index),
            data: rowsWithBreaksForGaps(data, COMBINED_LINE_BREAK_GAP_MS),
        }),
    );

    $: combinedSeries = rawCombinedSeries;

    let resolvedTotal = 0;
    let peakOnline24h = 0;
    let themeLabel = "System";
    let themeIconComponent = IconDeviceDesktop;
    let rankedServers: RankedServer[] = [];

    $: resolvedTotal = totalOnline(servers);
    $: peakOnline24h = totalPeak24h(servers);
    $: rankedServers = rankServersByOnline(servers);
    $: themeLabel = modeLabel(themeMode);
    $: themeIconComponent =
        themeMode === "light"
            ? IconSun
            : themeMode === "dark"
              ? IconMoon
              : IconDeviceDesktop;

    onMount(() => {
        const saved = (window.localStorage.getItem("minestats-theme") ??
            "system") as ThemeMode;
        applyTheme(
            saved === "light" || saved === "dark" || saved === "system"
                ? saved
                : "system",
        );

        const media = window.matchMedia("(prefers-color-scheme: dark)");
        const onMedia = () => {
            if (themeMode === "system") applyTheme("system");
        };
        media.addEventListener("change", onMedia);

        void refresh();
        const timer = window.setInterval(refresh, 2000);

        return () => {
            media.removeEventListener("change", onMedia);
            window.clearInterval(timer);
        };
    });
</script>

<main class="dashboard-shell">
    <section class="hero-card">
        <div class="hero-toolbar">
            <p class="eyebrow">MineStats</p>
        </div>

        <div class="hero-topline">
            <h1>Online Tracker</h1>
        </div>

        {#if loading}
            <div class="loader-inline" role="status" aria-live="polite">
                <span class="spinner"></span>
                <span>Loading live data...</span>
            </div>
        {/if}

        <p class="hero-description">
            Counting {resolvedTotal} players on {servers.length} Minecraft servers.
        </p>

        <div class="hero-metrics">
            <article>
                <div class="metric-label">
                    <span class="metric-icon" aria-hidden="true">
                        <IconUsers size={15} stroke={2} />
                    </span>
                    <p>Total Online</p>
                </div>
                <strong>{resolvedTotal}</strong>
            </article>

            <article>
                <div class="metric-label">
                    <span class="metric-icon" aria-hidden="true">
                        <IconServer size={15} stroke={2} />
                    </span>
                    <p>Tracked Servers</p>
                </div>
                <strong>{servers.length}</strong>
            </article>

            <article>
                <div class="metric-label">
                    <span class="metric-icon" aria-hidden="true">
                        <IconTrendingUp size={15} stroke={2} />
                    </span>
                    <p>24h Peak Online</p>
                </div>
                <strong>{peakOnline24h}</strong>
            </article>
        </div>

        <div class="combined-chart">
            <div
                class="chart-range-switcher"
                role="group"
                aria-label="Chart time range"
            >
                {#each ranges as range}
                    <button
                        type="button"
                        class={selectedRange === range.key ? "is-active" : ""}
                        on:click={() => setRange(range.key)}
                        >{range.label}</button
                    >
                {/each}
            </div>
            <CombinedChart
                series={combinedSeries}
                dark={isDark}
                rangeKey={selectedRange}
            />
        </div>
    </section>

    <section class="grid-cards">
        {#each rankedServers as ranked (ranked.server.id)}
            {@const server = ranked.server}
            {@const index = ranked.originalIndex}
            {@const record = topRecord(server, index)}
            <article class="server-card">
                <header>
                    <div class="server-header-main">
                        <div class="server-favicon">
                            {#if server.last_sample?.favicon}
                                <img
                                    src={server.last_sample.favicon}
                                    alt={`${server.name} favicon`}
                                />
                            {:else}
                                <span
                                    class="server-favicon-fallback"
                                    aria-hidden="true"
                                ></span>
                            {/if}
                        </div>
                        <div>
                            <h2>{server.name}</h2>
                            <p>{formatServerAddress(server.address)}</p>
                        </div>
                    </div>
                    <span>{status(server, index)}</span>
                </header>

                <div class="sparkline-wrap">
                    <ServerMiniChart
                        name={server.name}
                        data={numericRows(
                            resolveSeriesRows(miniSeries, server, index),
                        )}
                        dark={isDark}
                        color={serverColor(index)}
                    />
                </div>

                <div class="server-extra-stats">
                    <div>
                        <small>24h Peak</small>
                        <strong>{peak24h(server, index) ?? "-"}</strong>
                    </div>
                    <div>
                        <small>Top Record</small>
                        <strong>{record?.value ?? "-"}</strong>
                        <em>{record ? formatTopRecordTime(record.ts) : "-"}</em>
                    </div>
                </div>
            </article>
        {/each}
    </section>

    {#if errorMsg}
        <div
            class="api-outage-overlay"
            role="status"
            aria-live="polite"
            title={errorMsg}
        >
            <div class="api-outage-card">
                <p>Connection lost. Reconnecting...</p>
                <div class="api-status-track" aria-hidden="true">
                    <span class="api-status-slider"></span>
                </div>
            </div>
        </div>
    {/if}

    <footer class="dashboard-footer">
        <div class="footer-text">
            <p class="footer-meta">Made with ❤️</p>
            <p class="footer-disclaimer">
                This website is not an official Minecraft website and is not
                associated with Mojang Studios or Microsoft.
            </p>
        </div>
        <div class="footer-actions">
            <a
                class="github-link"
                href="https://github.com/adabugra/minestats"
                target="_blank"
                rel="noreferrer noopener"
                aria-label="View on GitHub"
            >
                <IconBrandGithub size={18} stroke={2} />
            </a>
            <div class="theme-switcher" aria-label="Theme switcher">
                <button
                    type="button"
                    on:click={cycleTheme}
                    class="theme-cycle-btn"
                >
                    <svelte:component
                        this={themeIconComponent}
                        size={14}
                        stroke={2}
                    />
                    <span>Theme: {themeLabel}</span>
                </button>
            </div>
        </div>
    </footer>
</main>
