<script>
import { Badge } from 'flowbite-svelte';
export let peers;
</script>

<div class="overflow-x-auto container mx-auto justify-center">
    <table class="w-full mx-auto table-lg table">
      <!-- head -->
      <thead>
        <tr>
          <th>#</th>
          <th>MachineID</th>
         <th>Hostname</th>
          <th>IP</th>
          <th>Connected</th>
          <th>Settings</th>
        </tr>
      </thead>
      <tbody>
        {#each peers as p, i}
        <tr>
            <td>{i}</td>
            <td>{p.machine_id}</td>
            <td>{p.Hostname}</td>
             <td>{p.ip}</td>
          <td>
            {#if true}
            <div class="badge variant-filled-success"></div>
            {:else}
            <div class="badge variant-filled-warning"></div>
            {/if}
          </td>
          <td>
            <button class="btn btn-square btn-sm btn-ghost">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 20 20" class="inline-block w-4 h-4 stroke-current"><path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 12h.01M12 12h.01M19 12h.01M6 12a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0zm7 0a1 1 0 11-2 0 1 1 0 012 0z"></path></svg>
              </button>
          </td>
        </tr>
        {/each}
      </tbody>
    </table>
  </div>

  <style>
    td {
        text-align: center;
    }
    th {
        text-align: center;
    }

  </style>

