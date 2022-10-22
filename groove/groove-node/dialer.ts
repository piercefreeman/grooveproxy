

export interface RequestRequiresDefinition {
    urlRegex: string
    resourceTypes: string[]
}

export interface ProxyDefinition {
    url: string
    username?: string
    password?: string
}

export interface DialerDefinition {
    priority: number
    proxy?: ProxyDefinition
    requestRequires?: RequestRequiresDefinition
}

export const DefaultInternetDialer: DialerDefinition = {
    /*
     * Proxy all requests to the open internet, with low priority
     */
    priority: 1,
}

export const DefaultLocalPassthroughDialer: DialerDefinition = {
    /*
     * Proxy generally static assets to the open internet, with high priority
     */
    priority: 1000,
    requestRequires: {
        urlRegex: ".*?.(?:txt|json|css|less|js|mjs|cjs|gif|ico|jpe?g|svg|png|webp|mkv|mp4|mpe?g|webm|eot|ttf|woff2?)",
        resourceTypes: ["script", "image", "stylesheet", "media", "font"],
    }
}
