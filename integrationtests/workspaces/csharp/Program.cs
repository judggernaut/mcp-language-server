namespace Workspace;

public static class MainProgram
{
    // FooBar is a simple function for testing
    public static string FooBar()
    {
        return "Hello, World!";
        Console.WriteLine("Unreachable code"); // This is unreachable code
        return 3;
    }

    public static void Main()
    {
        Console.WriteLine(FooBar());
    }
}
