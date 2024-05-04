<script lang="ts">
import {user, token, isAuthenticated} from '$lib/stores/auth'
import Peers from '$lib/components/peers.svelte'
export let data;

</script>
<!-- <div>{JSON.stringify(data)}</div> -->


<div class="container h-full mx-auto flex justify-center items-center">
    <Peers peers={data.peers}/>
</div>
