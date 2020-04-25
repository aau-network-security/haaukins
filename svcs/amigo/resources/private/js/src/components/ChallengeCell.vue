<template>
    <td class="text-center scoreboard-border">
        <span class="icon" v-bind:class="{ 'text-success': completed }">
            <i class="fas" v-bind:class="{ 'fa-check-circle': completed, 'fa-circle': !completed }"></i>
        </span>
    </td>
</template>

<script>
    /* eslint-disable */
    export default {
        name: 'challenge-cell',
        props: {
            completed: Boolean,
        },
    }
</script>
