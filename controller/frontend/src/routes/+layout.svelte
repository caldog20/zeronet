<script lang='ts'>
	import '../app.postcss';
	import { AppShell, AppBar, Avatar } from '@skeletonlabs/skeleton';
  import Nav from '$lib/components/nav.svelte'
  import { Drawer, getDrawerStore } from '@skeletonlabs/skeleton';
  import type { DrawerSettings, DrawerStore } from '@skeletonlabs/skeleton';
  import {onMount} from 'svelte'
	import { initializeStores } from '@skeletonlabs/skeleton';
  import auth from '$lib/services/auth';
  import {user, token, isAuthenticated, popupOpen} from '$lib/stores/auth'


	// Highlight JS
	import hljs from 'highlight.js/lib/core';
	import 'highlight.js/styles/github-dark.css';
	import { storeHighlightJs } from '@skeletonlabs/skeleton';
	import xml from 'highlight.js/lib/languages/xml'; // for HTML
	import css from 'highlight.js/lib/languages/css';
	import javascript from 'highlight.js/lib/languages/javascript';
	import typescript from 'highlight.js/lib/languages/typescript';

	hljs.registerLanguage('xml', xml); // for HTML
	hljs.registerLanguage('css', css);
	hljs.registerLanguage('javascript', javascript);
	hljs.registerLanguage('typescript', typescript);
	storeHighlightJs.set(hljs);

	// Floating UI for Popups
	import { computePosition, autoUpdate, flip, shift, offset, arrow } from '@floating-ui/dom';
	import { storePopup } from '@skeletonlabs/skeleton';
	storePopup.set({ computePosition, autoUpdate, flip, shift, offset, arrow });


	let auth0Client;
	onMount(async () => {
		auth0Client = await auth.createClient();
		isAuthenticated.set(await auth0Client.isAuthenticated());
		user.set(await auth0Client.getUser());
	});

	function login() {
		auth.loginWithPopup(auth0Client);
	}

	function logout() {
		auth.logout(auth0Client);
	}

  initializeStores();
	const drawerStore = getDrawerStore();
	
	function drawerOpen(): void {
		drawerStore.open();
	}

  function drawerClose(): void {
		drawerStore.close();
	}


// export const isAuthed = await auth0.isAuthenticated();

</script> 
{#if !$isAuthenticated}
  <div class="loginButton center">
          <a
            class="btn btn-primary btn-lg mr-auto ml-auto variant-ghost-surface"
            href="/#"
            role="button"
            on:click="{login}"
            >Log In</a
          >
        </div>
  {:else}

<Drawer>
	<Nav/>
</Drawer>

<AppShell slotSidebarLeft="w-0 md:w-52 bg-surface-500/10">
	<svelte:fragment slot="header">
		<AppBar>
			<svelte:fragment slot="lead">
				<button class="md:hidden btn btn-sm mr-4" on:click={drawerOpen}>
					<span>
						<svg viewBox="0 0 100 80" class="fill-token w-4 h-4">
							<rect width="100" height="20" />
							<rect y="30" width="100" height="20" />
							<rect y="60" width="100" height="20" />
						</svg>
					</span>
				</button>
				<strong class="text-xl">ZeroNet Dashboard</strong>
			</svelte:fragment>
			<svelte:fragment slot="trail">
				<Avatar src={$user.picture} background="bg-primary-500" width="w-12" />
        <button class="btn btm-sm variant-ghost-surface" on:click={logout}>Logout</button>
				<!-- <a -->
				<!-- 	class="btn btn-sm variant-ghost-surface" -->
				<!-- 	href="https://github.com/skeletonlabs/skeleton" -->
				<!-- 	target="_blank" -->
				<!-- 	rel="noreferrer" -->
				<!-- > -->
				<!-- 	GitHub -->
				<!-- </a> -->
			</svelte:fragment>
			<!-- <svelte:fragment slot="headline">(headline)</svelte:fragment> -->
		</AppBar>
	</svelte:fragment>
	<svelte:fragment slot="sidebarLeft"><Nav></Nav></svelte:fragment>
	<!-- (sidebarRight) -->
	<!-- <svelte:fragment slot="pageHeader">Page Header</svelte:fragment> -->
	<!-- Router Slot -->
	<div class="container p-10 mx-auto">
		<slot />
	</div>
	
	<!-- ---- / ---- -->
	<!-- <svelte:fragment slot="pageFooter">Page Footer</svelte:fragment> -->
	<!-- (footer) -->
</AppShell>


{/if}

<style>
.loginButton {
  display: flex; /* or grid */
  justify-content: center;
  align-items: center;
}

.center{
      margin-top: 20%;
      text-align: center;
    }
</style>
