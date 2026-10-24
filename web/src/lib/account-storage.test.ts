import test from "node:test";
import assert from "node:assert/strict";
import { accountStorageKey, accountStateStorage, accountBlobStorage } from "./account-storage";
import { localForageStorage } from "./localforage-storage";
import { useUserStore } from "@/stores/use-user-store";
import type { AuthUser } from "@/services/api/auth";

function account(id: string) {
    useUserStore.setState({ isReady: true, token: `session-${id}`, user: { id, role: "user" } as AuthUser });
}

test("guest cannot access cached business data or write media", async () => {
    useUserStore.setState({ isReady: true, token: "", user: null });
    assert.equal(accountStorageKey("canvas"), null);
    assert.equal(await accountStateStorage.getItem("canvas"), null);
    assert.throws(() => accountBlobStorage("image_files").setItem("image", "data"), /登录/);
});

test("two accounts have distinct caches and legacy data is never assigned or deleted", async () => {
    const original = { ...localForageStorage };
    const cache = new Map([["canvas", "legacy-private-project"]]);
    localForageStorage.getItem = (key) => cache.get(key) ?? null;
    localForageStorage.setItem = (key, value) => { cache.set(key, value); };
    localForageStorage.removeItem = (key) => { cache.delete(key); };
    try {
        account("alice");
        assert.equal(await accountStateStorage.getItem("canvas"), null);
        await accountStateStorage.setItem("canvas", "alice-project");
        account("bob");
        // A stale store cannot save Alice's in-memory state under Bob's key.
        await accountStateStorage.setItem("canvas", "stale-alice-project");
        assert.equal(await accountStateStorage.getItem("canvas"), null);
        await accountStateStorage.setItem("canvas", "bob-project");
        account("alice");
        assert.equal(await accountStateStorage.getItem("canvas"), "alice-project");
        await accountStateStorage.removeItem("canvas");
        assert.equal(cache.get("canvas:account:bob"), "bob-project");
        assert.equal(cache.get("canvas"), "legacy-private-project");
    } finally { Object.assign(localForageStorage, original); }
});

test("pending hydration cannot expose an account's cache after switching users", async () => {
    const original = localForageStorage.getItem;
    let release!: (value: string) => void;
    localForageStorage.getItem = () => new Promise<string>(resolve => { release = resolve; });
    try {
        account("alice");
        const pending = accountStateStorage.getItem("delayed-canvas");
        await Promise.resolve();
        account("bob");
        release("alice-project");
        assert.equal(await pending, null);
    } finally { localForageStorage.getItem = original; }
});
