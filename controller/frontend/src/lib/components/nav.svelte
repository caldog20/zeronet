<script lang="ts">
    import { getDrawerStore } from '@skeletonlabs/skeleton';
    import type { DrawerSettings, DrawerStore } from '@skeletonlabs/skeleton';
    
    const drawerStore = getDrawerStore();
    function drawerClose(): void {
		drawerStore.close();
	}
</script>

<nav class="list-nav p-4">
    <ul>
        <li><a href="/" on:click={drawerClose}>Peers</a></li>
        <li><a href="/about"  on:click={drawerClose}>About</a></li>
    </ul>
</nav>
