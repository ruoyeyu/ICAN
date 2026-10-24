import localforage from "localforage";
import type { StateStorage } from "zustand/middleware";
import { useUserStore } from "@/stores/use-user-store";
import { localForageStorage } from "@/lib/localforage-storage";

export function accountStorageKey(name: string) {
    const { user, isReady, token } = useUserStore.getState();
    return isReady && token && user && user.role !== "guest" ? `${name}:account:${user.id}` : null;
}

export async function waitForAccount() {
    if (useUserStore.getState().isReady) return;
    await new Promise<void>((resolve) => {
        const unsubscribe = useUserStore.subscribe((state) => {
            if (state.isReady) { unsubscribe(); resolve(); }
        });
    });
}

// Legacy, unowned keys are preserved for explicit recovery, never silently
// attributed or uploaded to whichever account happens to log in next.
const loadedKeys = new Map<string, string>();

export const accountStateStorage: StateStorage = {
    getItem: async (name) => {
        await waitForAccount();
        const key = accountStorageKey(name);
        if (!key) return null;
        const value = await localForageStorage.getItem(key);
        if (accountStorageKey(name) === key) loadedKeys.set(name, key);
        return accountStorageKey(name) === key ? value : null;
    },
    setItem: (name, value) => {
        const key = accountStorageKey(name);
        if (key && loadedKeys.get(name) === key) return localForageStorage.setItem(key, value);
    },
    removeItem: (name) => {
        const key = accountStorageKey(name);
        if (key) return localForageStorage.removeItem(key);
    },
};

export function accountBlobStorage(storeName: string) {
    const stores = new Map<string, ReturnType<typeof localforage.createInstance>>();
    function current() {
        const key = accountStorageKey(storeName);
        if (!key) throw new Error("请先登录后再访问素材");
        if (!stores.has(key)) stores.set(key, localforage.createInstance({ name: `ican:${key}`, storeName }));
        return stores.get(key)!;
    }
    return {
        getItem: <T>(key: string) => current().getItem<T>(key),
        setItem: <T>(key: string, value: T) => current().setItem(key, value),
        removeItem: (key: string) => current().removeItem(key),
        iterate: <T, U>(callback: (value: T, key: string, iterationNumber: number) => U) => current().iterate(callback),
    };
}
