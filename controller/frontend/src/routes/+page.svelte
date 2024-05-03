<script lang="ts">
import {user, token, isAuthenticated} from '$lib/stores/auth'
</script>

{#if !$isAuthenticated}
<h1>Hey {$user.name}</h1>

{/if}

